package handlers

import (
	"context"
	"errors"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/util"
	"gorm.io/gorm/clause"
	"net/http"
	"strings"

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

// ListSecurityGroups lists all Security Groups
// @Summary      List Security Groups
// @Description  Lists all Security Groups
// @Id  		 ListSecurityGroups
// @Tags         SecurityGroup
// @Accepts		 json
// @Produce      json
// @Param		 gt_revision       query     uint64 false "greater than revision"
// @Param        organization_id   path      string  true "Organization ID"
// @Success      200  {object}  []models.SecurityGroup
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/organizations/{organization_id}/security_groups [get]
func (api *API) ListSecurityGroups(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListSecurityGroups")
	defer span.End()

	orgId, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var org models.Organization
	db := api.db.WithContext(ctx)
	if res := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", orgId); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	var query Query
	if err := c.BindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiInternalError(err))
		return
	}

	api.sendList(c, ctx, func(db *gorm.DB) (fetchmgr.ResourceList, error) {
		var items securityGroupList
		db = db.Where("organization_id = ?", orgId)
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
// @Description  Gets a security group in an organization by ID
// @Id  		 GetSecurityGroup
// @Tags         SecurityGroup
// @Accepts		 json
// @Produce      json
// @Param        organization_id   path      string  true "Organization ID"
// @Param        id   path      string  true "Security Group ID"
// @Success      200  {object}  models.SecurityGroup
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/organizations/{organization_id}/security_group/{id} [get]
func (api *API) GetSecurityGroup(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetSecurityGroup", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
		attribute.String("organization", c.Param("organization")),
	))
	defer span.End()
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	orgId, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var org models.Organization
	db := api.db.WithContext(ctx)
	if res := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", orgId); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	var securityGroup models.SecurityGroup
	result := db.
		First(&securityGroup, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, securityGroup)
}

func (api *API) secGroupsEnabled(c *gin.Context) bool {
	secGroupsEnabled, err := api.fflags.GetFlag("security-groups")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return false
	}
	allowForTests := c.GetString("nexodus.secGroupsEnabled")
	if (!secGroupsEnabled && allowForTests != "true") || allowForTests == "false" {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("security-groups support is disabled"))
		return false
	}
	return true
}

// CreateSecurityGroup handles adding a new SecurityGroup
// @Summary      Add SecurityGroup
// @Id  		 CreateSecurityGroup
// @Tags         SecurityGroup
// @Description  Adds a new Security Group
// @Accepts		 json
// @Produce      json
// @Param        organization_id   path      string  true "Organization ID"
// @Param        SecurityGroup   body   models.AddSecurityGroup  true "Add SecurityGroup"
// @Success      201  {object}  models.SecurityGroup
// @Failure      400  {object}  models.BaseError
// @Failure      401  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure      422  {object}  models.ValidationError
// @Failure      429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /api/organizations/{organization_id}/security_groups [post]
func (api *API) CreateSecurityGroup(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateSecurityGroup", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
		attribute.String("organization", c.Param("organization")),
	))
	defer span.End()

	if !api.secGroupsEnabled(c) {
		return
	}

	var request models.AddSecurityGroup
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
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

	if request.OrganizationId == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("org_id"))
		return
	}

	if request.GroupName == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("group_name"))
		return
	}

	var sg models.SecurityGroup
	err := api.transaction(ctx, func(tx *gorm.DB) error {
		var org models.Organization
		if res := api.OrganizationIsOwnedByCurrentUser(c, tx).
			First(&org, "id = ?", request.OrganizationId); res.Error != nil {
			return res.Error
		}

		sg = models.SecurityGroup{
			GroupName:        request.GroupName,
			OrganizationId:   request.OrganizationId,
			InboundRules:     request.InboundRules,
			OutboundRules:    request.OutboundRules,
			GroupDescription: request.GroupDescription,
		}
		if res := tx.Create(&sg); res.Error != nil {
			return res.Error
		}

		// Replace the organization's SecurityGroupId field
		org.SecurityGroupId = sg.ID
		if res := tx.Save(&org); res.Error != nil {
			return res.Error
		}

		span.SetAttributes(attribute.String("id", sg.ID.String()))
		api.logger.Infof("New security group created [ %s ] in organization [ %s ]", sg.GroupName, org.ID)
		return nil
	})

	if err != nil {
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewApiInternalError(err))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
	}

	signalChannel := fmt.Sprintf("/security-groups/org=%s", sg.OrganizationId.String())
	api.signalBus.Notify(signalChannel)

	c.JSON(http.StatusCreated, sg)
}

// DeleteSecurityGroup handles deleting an existing security group
// @Summary      Delete SecurityGroup
// @Description  Deletes an existing SecurityGroup
// @Id 			 DeleteSecurityGroup
// @Tags         SecurityGroup
// @Accepts		 json
// @Produce      json
// @Param        organization_id   path      string  true "Organization ID"
// @Param        security_group_id   path      string  true "Security Group ID"
// @Success      204  {object}  models.SecurityGroup
// @Failure      400  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /api/organizations/{organization_id}/security_groups/{security_group_id} [delete]
func (api *API) DeleteSecurityGroup(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteSecurityGroup", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
		attribute.String("organization", c.Param("organization")),
	))

	defer span.End()

	if !api.secGroupsEnabled(c) {
		return
	}

	secGroupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	orgId, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	sg := models.SecurityGroup{}

	err = api.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var organization models.Organization
		result := api.OrganizationIsOwnedByCurrentUser(c, tx).
			First(&organization, "id = ?", orgId.String())
		if result.Error != nil {
			return result.Error
		}

		if res := tx.First(&sg, "id = ?", secGroupID); res.Error != nil {
			if result.Error != nil {
				return result.Error
			}
		}

		if res := tx.Delete(&sg, "id = ?", sg.ID); res.Error != nil {
			return result.Error
		}

		if res := tx.Model(&models.Organization{}).
			Where("security_group_id = ?", sg.ID).
			Update("security_group_id", nil); res.Error != nil {
			return result.Error
		}

		if res := tx.Model(&models.Device{}).
			Where("organization_id = ? AND security_group_id = ?", sg.OrganizationId, sg.ID).
			Update("security_group_id", nil); res.Error != nil {
			return result.Error
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, err)
		} else {
			c.JSON(http.StatusInternalServerError, err)
		}
		return
	}

	signalChannel := fmt.Sprintf("/security-groups/org=%s", sg.OrganizationId.String())
	api.signalBus.Notify(signalChannel)

	c.JSON(http.StatusOK, sg)
}

// UpdateSecurityGroup updates a Security Group
// @Summary      Update Security Group
// @Description  Updates a Security Group by ID
// @Id           UpdateSecurityGroup
// @Tags         SecurityGroup
// @Accepts      json
// @Produce      json
// @Param        organization_id   path      string  true "Organization ID"
// @Param        security_group_id path      string  true "Security Group ID"
// @Param        update body       models.UpdateSecurityGroup true "Security Group Update"
// @Success      200  {object}     models.SecurityGroup
// @Failure      400  {object}     models.BaseError
// @Failure      401  {object}     models.BaseError
// @Failure      404  {object}     models.BaseError
// @Failure      422  {object}     models.ValidationError
// @Failure      429  {object}     models.BaseError
// @Router       /api/organizations/{organization_id}/security_groups/{security_group_id} [patch]
func (api *API) UpdateSecurityGroup(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateSecurityGroup", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()

	if !api.secGroupsEnabled(c) {
		return
	}

	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	orgId, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var request models.UpdateSecurityGroup
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
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
		var org models.Organization
		if res := api.OrganizationIsOwnedByCurrentUser(c, tx).
			First(&org, "id = ?", orgId); res.Error != nil {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("security_group"))
			return res.Error
		}

		result := tx.
			First(&securityGroup, "id = ?", k)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return errSecurityGroupNotFound
		}

		securityGroup.GroupName = request.GroupName
		securityGroup.GroupDescription = request.GroupDescription
		securityGroup.InboundRules = request.InboundRules
		securityGroup.OutboundRules = request.OutboundRules

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
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
	}

	signalChannel := fmt.Sprintf("/security-groups/org=%s", securityGroup.OrganizationId.String())
	api.signalBus.Notify(signalChannel)

	c.JSON(http.StatusOK, securityGroup)
}

// createDefaultSecurityGroup creates the default security group for the organization
func (api *API) createDefaultSecurityGroup(ctx context.Context, db *gorm.DB, orgId string) (models.SecurityGroup, error) {
	orgIdUUID, err := uuid.Parse(orgId)
	if err != nil {
		return models.SecurityGroup{}, err
	}

	var inboundRules []models.SecurityRule
	var outboundRules []models.SecurityRule

	// Create the default security group
	sg := models.SecurityGroup{
		GroupName:        "default",
		OrganizationId:   orgIdUUID,
		GroupDescription: "default organization security group",
		InboundRules:     inboundRules,
		OutboundRules:    outboundRules,
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

// updateOrganizationSecGroupId updates the security group ID in an org entry
func (api *API) updateOrganizationSecGroupId(ctx context.Context, db *gorm.DB, sgId, orgId uuid.UUID) error {
	var org models.Organization
	if db == nil {
		db = api.db.WithContext(ctx)
	}

	res := db.WithContext(ctx).First(&org, "id = ?", orgId)
	if res.Error != nil {
		return res.Error
	}

	return db.WithContext(ctx).Model(&org).Update("SecurityGroupId", sgId).Error
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
	if rule.FromPort != 0 || rule.ToPort != 0 { // Checks if they are set (i.e., not wildcard)
		if rule.FromPort > rule.ToPort || rule.FromPort < 0 || rule.ToPort > 65535 {
			return fmt.Errorf("invalid port range: from %d to %d", rule.FromPort, rule.ToPort)
		}
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
