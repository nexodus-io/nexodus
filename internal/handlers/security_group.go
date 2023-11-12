package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/util"
	"gorm.io/gorm/clause"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const (
	// Protocols
	protoIPv4   = "ipv4"
	protoIPv6   = "ipv6"
	protoICMPv4 = "icmpv4"
	protoICMP   = "icmp"
	protoICMPv6 = "icmpv6"
	protoTCP    = "tcp"
	protoUDP    = "udp"
)

var allowedProtocols = map[string]bool{
	protoIPv4:   true,
	protoIPv6:   true,
	protoICMPv4: true,
	protoICMP:   true,
	protoICMPv6: true,
	protoTCP:    true,
	protoUDP:    true,
}

type securityGroupList []*models.SecurityGroup

func (d securityGroupList) Item(i int) (any, uint64, gorm.DeletedAt) {
	item := d[i]
	return item, item.Revision, item.DeletedAt
}

func (d securityGroupList) Len() int {
	return len(d)
}

func (api *API) SecurityGroupIsReadableByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	if api.dialect == database.DialectSqlLite {
		return db.Where("organization_id in (SELECT organization_id FROM user_organizations where user_id=?) OR organization_id in (SELECT id FROM organizations where owner_id=?)", userId, userId)
	} else {
		return db.Where("organization_id::text in (SELECT organization_id::text FROM user_organizations where user_id=?) OR organization_id::text in (SELECT id::text FROM organizations where owner_id=?)", userId, userId)
	}
}

func (api *API) SecurityGroupIsWriteableByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	if api.dialect == database.DialectSqlLite {
		return db.Where("organization_id in (SELECT id FROM organizations where owner_id=?)", userId)
	} else {
		return db.Where("organization_id::text in (SELECT id::text FROM organizations where owner_id=?)", userId)
	}
}

// ListSecurityGroups lists all Security Groups
// @Summary      List Security Groups
// @Description  Lists all Security Groups
// @Id  		 ListSecurityGroups
// @Tags         SecurityGroup
// @Accepts		 json
// @Produce      json
// @Param		 gt_revision       query     uint64 false "greater than revision"
// @Success      200  {object}  []models.SecurityGroup
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/security-groups [get]
func (api *API) ListSecurityGroups(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListSecurityGroups")
	defer span.End()

	var query Query
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiError(err))
		return
	}

	api.sendList(c, ctx, func(db *gorm.DB) (fetchmgr.ResourceList, error) {
		var items securityGroupList

		err := api.transaction(ctx, func(tx *gorm.DB) error {
			vpcs := []models.VPC{}
			if result := api.VPCIsReadableByCurrentUser(c, tx).
				Preload("Organization").Find(&vpcs); result.Error != nil {
				return result.Error
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		db = api.SecurityGroupIsReadableByCurrentUser(c, db)
		db = FilterAndPaginateWithQuery(db, &models.SecurityGroup{}, c, query, "description")
		result := db.Find(&items)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
		return items, nil
	})
}

// ListSecurityGroupsInVPC lists all Security Groups in a VPC
// @Summary      List Security Groups in a VPC
// @Description  Lists all Security Groups in a VPC
// @Id  		 ListSecurityGroupsInVPC
// @Tags         VPC
// @Accepts		 json
// @Produce      json
// @Param		 gt_revision       query     uint64 false "greater than revision"
// @Param        id                path      string  true "VPC ID"
// @Success      200  {object}  []models.SecurityGroup
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs/{id}/security-groups [get]
func (api *API) ListSecurityGroupsInVPC(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListSecurityGroupsInVPC",
		trace.WithAttributes(
			attribute.String("vpc_id", c.Param("id")),
		))
	defer span.End()

	vpcId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var vpc models.VPC
	db := api.db.WithContext(ctx)
	result := api.VPCIsReadableByCurrentUser(c, db).
		First(&vpc, "id = ?", vpcId.String())
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("vpc"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	var query Query
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiError(err))
		return
	}

	api.sendList(c, ctx, func(db *gorm.DB) (fetchmgr.ResourceList, error) {
		var items securityGroupList

		if api.dialect == database.DialectSqlLite {
			db = db.Where("organization_id in (SELECT DISTINCT organization_id FROM devices where vpc_id=?)", vpcId.String())
		} else {
			db = db.Where("organization_id::text in (SELECT DISTINCT organization_id::text FROM devices where vpc_id=?)", vpcId.String())
		}

		db = FilterAndPaginateWithQuery(db, &models.SecurityGroup{}, c, query, "id")
		result := db.Find(&items)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
		return items, nil
	})
}

// GetSecurityGroup gets a Security Group by ID
// @Summary      Get SecurityGroup
// @Description  Gets a security group by ID
// @Id  		 GetSecurityGroup
// @Tags         SecurityGroup
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Security Group ID"
// @Success      200  {object}  models.SecurityGroup
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/security-groups/{id} [get]
func (api *API) GetSecurityGroup(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetSecurityGroup", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	db := api.db.WithContext(ctx)
	db = api.SecurityGroupIsReadableByCurrentUser(c, db)
	var securityGroup models.SecurityGroup
	result := db.
		First(&securityGroup, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, securityGroup)
}

// CreateSecurityGroup handles adding a new SecurityGroup
// @Summary      Add SecurityGroup
// @Id  		 CreateSecurityGroup
// @Tags         SecurityGroup
// @Description  Adds a new Security Group
// @Accepts		 json
// @Produce      json
// @Param        SecurityGroup   body   models.AddSecurityGroup  true "Add SecurityGroup"
// @Success      201  {object}  models.SecurityGroup
// @Failure      400  {object}  models.BaseError
// @Failure      401  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure      422  {object}  models.ValidationError
// @Failure      429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/security-groups [post]
func (api *API) CreateSecurityGroup(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateSecurityGroup")
	defer span.End()

	if !api.FlagCheck(c, "security-groups") {
		return
	}

	var request models.AddSecurityGroup
	// Call BindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	// Validate security group rules for any invalid fields in ports/ip_ranges/protocol
	if err := ValidateCreateSecurityGroupRules(request); err != nil {
		switch {
		case strings.Contains(err.Error(), "invalid protocol"):
			c.JSON(http.StatusUnprocessableEntity, models.NewFieldValidationError("protocol", err.Error()))
		case strings.Contains(err.Error(), "invalid port range"):
			c.JSON(http.StatusUnprocessableEntity, models.NewFieldValidationError("port_range", err.Error()))
		case strings.Contains(err.Error(), "invalid IP range"):
			c.JSON(http.StatusUnprocessableEntity, models.NewFieldValidationError("ip_range", err.Error()))
		default:
			c.JSON(http.StatusUnprocessableEntity, models.NewFieldValidationError("rule", "invalid rule"))
		}
		return
	}

	if request.VpcId == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("vpc_id"))
		return
	}

	var sg models.SecurityGroup
	err := api.transaction(ctx, func(tx *gorm.DB) error {
		var vpc models.VPC
		if res := api.VPCIsOwnedByCurrentUser(c, tx).
			First(&vpc, "id = ?", request.VpcId); res.Error != nil {
			return res.Error
		}

		sg = models.SecurityGroup{
			VpcId:          vpc.ID,
			OrganizationID: vpc.OrganizationID,
			InboundRules:   request.InboundRules,
			OutboundRules:  request.OutboundRules,
			Description:    request.Description,
		}
		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Create(&sg); res.Error != nil {
			return res.Error
		}

		span.SetAttributes(attribute.String("id", sg.ID.String()))
		api.logger.Infof("New security group created [ %s ] in organization [ %s ]", sg.ID, vpc.ID)
		return nil
	})

	if err != nil {
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewApiError(err))
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	api.notifySecurityGroupChange(c, sg.VpcId)
	c.JSON(http.StatusCreated, sg)
}

func (api *API) notifySecurityGroupChange(c *gin.Context, orgId uuid.UUID) {
	vpcIds := []uuid.UUID{}
	db := api.db.WithContext(c)
	result := db.Model(&models.VPC{}).
		Where("organization_id = ?", orgId).
		Distinct().
		Pluck("id", &vpcIds)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		api.logger.Errorf("Failed to fetch vpc ids for organization %s: %s", orgId, result.Error)
		return
	}

	for _, id := range vpcIds {
		signalChannel := fmt.Sprintf("/security-groups/vpc=%s", id.String())
		api.signalBus.Notify(signalChannel)
	}
}

// DeleteSecurityGroup handles deleting an existing security group
// @Summary      Delete SecurityGroup
// @Description  Deletes an existing SecurityGroup
// @Id 			 DeleteSecurityGroup
// @Tags         SecurityGroup
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Security Group ID"
// @Success      204  {object}  models.SecurityGroup
// @Failure      400  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/security-groups/{id} [delete]
func (api *API) DeleteSecurityGroup(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteSecurityGroup", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))

	defer span.End()

	if !api.FlagCheck(c, "security-groups") {
		return
	}

	secGroupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	sg := models.SecurityGroup{}
	err = api.transaction(ctx, func(tx *gorm.DB) error {

		if res := api.SecurityGroupIsWriteableByCurrentUser(c, tx).
			First(&sg, "id = ?", secGroupID); res.Error != nil {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("vpc"))
		}

		var count int64
		res := tx.Model(&models.Device{}).Where("security_group_id = ?", secGroupID).Count(&count)
		if res.Error != nil {
			return res.Error
		}
		if count > 0 {
			return NewApiResponseError(http.StatusBadRequest, models.NewNotAllowedError("security group cannot be delete while devices are still using it"))
		}

		if sg.ID == sg.VpcId {
			return NewApiResponseError(http.StatusBadRequest, models.NewNotAllowedError("default security group cannot be deleted"))
		}

		if res = tx.Delete(&sg, "id = ?", sg.ID); res.Error != nil {
			return res.Error
		}

		if res := tx.Model(&models.Device{}).
			Where("vpc_id = ? AND security_group_id = ?", sg.VpcId, sg.ID).
			Update("security_group_id", nil); res.Error != nil {
			return res.Error
		}

		return nil
	})

	if err != nil {
		var apiResponseError *ApiResponseError
		if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, err)
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	api.notifySecurityGroupChange(c, sg.VpcId)

	c.JSON(http.StatusOK, sg)
}

// UpdateSecurityGroup updates a Security Group
// @Summary      Update Security Group
// @Description  Updates a Security Group by ID
// @Id           UpdateSecurityGroup
// @Tags         SecurityGroup
// @Accepts      json
// @Produce      json
// @Param        id path      string  true "Security Group ID"
// @Param        update body       models.UpdateSecurityGroup true "Security Group Update"
// @Success      200  {object}     models.SecurityGroup
// @Failure      400  {object}     models.BaseError
// @Failure      401  {object}     models.BaseError
// @Failure      404  {object}     models.BaseError
// @Failure      422  {object}     models.ValidationError
// @Failure      429  {object}     models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/security-groups/{id} [patch]
func (api *API) UpdateSecurityGroup(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateSecurityGroup", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()

	if !api.FlagCheck(c, "security-groups") {
		return
	}

	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var request models.UpdateSecurityGroup
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	// Validate security group rules for any invalid fields in ports/ip_ranges/protocol
	if err := ValidateUpdateSecurityGroupRules(request); err != nil {
		switch {
		case strings.Contains(err.Error(), "invalid protocol"):
			c.JSON(http.StatusUnprocessableEntity, models.NewFieldValidationError("protocol", err.Error()))
		case strings.Contains(err.Error(), "invalid port range"):
			c.JSON(http.StatusUnprocessableEntity, models.NewFieldValidationError("port_range", err.Error()))
		case strings.Contains(err.Error(), "invalid IP range"):
			c.JSON(http.StatusUnprocessableEntity, models.NewFieldValidationError("ip_range", err.Error()))
		default:
			c.JSON(http.StatusUnprocessableEntity, models.NewFieldValidationError("rule", "invalid rule"))
		}
		return
	}

	var securityGroup models.SecurityGroup
	err = api.transaction(ctx, func(tx *gorm.DB) error {

		result := api.SecurityGroupIsWriteableByCurrentUser(c, tx).
			First(&securityGroup, "id = ?", k)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return errSecurityGroupNotFound
		}

		if request.Description != nil {
			securityGroup.Description = *request.Description
		}
		if request.InboundRules != nil {
			securityGroup.InboundRules = request.InboundRules
		}
		if request.OutboundRules != nil {
			securityGroup.OutboundRules = request.OutboundRules
		}

		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Save(&securityGroup); res.Error != nil {
			return res.Error
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errSecurityGroupNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("security_group"))
		} else if errors.Is(err, errOrgNotFound) {
			c.JSON(http.StatusNotFound, err)
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	api.notifySecurityGroupChange(c, securityGroup.VpcId)

	c.JSON(http.StatusOK, securityGroup)
}

// createDefaultSecurityGroup creates the default security group for the organization
func (api *API) createDefaultSecurityGroup(ctx context.Context, db *gorm.DB, vpcId uuid.UUID, orgId uuid.UUID) (models.SecurityGroup, error) {

	// Create the default security group
	sg := models.SecurityGroup{
		Base: models.Base{
			ID: vpcId,
		},
		VpcId:          vpcId,
		OrganizationID: orgId,
		Description:    "default vpc security group",
		InboundRules:   []models.SecurityRule{},
		OutboundRules:  []models.SecurityRule{},
	}

	if db == nil {
		db = api.db.WithContext(ctx)
	}

	res := db.Create(&sg)
	if res.Error != nil {
		return models.SecurityGroup{}, fmt.Errorf("failed to create the default organization security group: %w", res.Error)
	}

	return sg, nil
}

// ValidateUpdateSecurityGroupRules validates rules for updating the security group
func ValidateUpdateSecurityGroupRules(sg models.UpdateSecurityGroup) error {
	for _, rule := range append(sg.InboundRules, sg.OutboundRules...) {
		if err := ValidateRule(rule); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCreateSecurityGroupRules validates rules for creating a new security group
func ValidateCreateSecurityGroupRules(sg models.AddSecurityGroup) error {
	for _, rule := range append(sg.InboundRules, sg.OutboundRules...) {
		if err := ValidateRule(rule); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRule validates individual rule
func ValidateRule(rule models.SecurityRule) error {
	// Validate Protocol
	if rule.IpProtocol != "" && !allowedProtocols[rule.IpProtocol] {
		return fmt.Errorf("invalid protocol: %s", rule.IpProtocol)
	}

	// Validate Ports
	if rule.FromPort == 0 && rule.ToPort == 0 {
		// Both ports are zero, which is a valid case
	} else if rule.FromPort == 0 || rule.ToPort == 0 {
		// If either port is 0, they both have to be zero
		return fmt.Errorf("invalid port range: port ranges need to have a value greater than 0")
	} else if rule.FromPort > rule.ToPort || rule.FromPort < 1 || rule.ToPort > 65535 {
		// from_port needs to be less than the to_port in the range of 1-65535
		return fmt.Errorf("invalid port range: from %d to %d", rule.FromPort, rule.ToPort)
	}

	// Validate IP Ranges
	for _, ipRange := range rule.IpRanges {
		if ipRange == "" { // Wildcard case
			continue
		}

		// Check the
		isIPv4 := util.ContainsValidCustomIPv4Ranges([]string{ipRange})
		isIPv6 := util.ContainsValidCustomIPv6Ranges([]string{ipRange})

		// If the IP range is neither a valid IPv4 nor a valid IPv6, then it's invalid
		if !isIPv4 && !isIPv6 {
			return fmt.Errorf("invalid IP range: %s", ipRange)
		}
	}

	return nil
}
