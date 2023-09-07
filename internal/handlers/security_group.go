package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

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
	if res := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		First(&org, "id = ?", orgId); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	var query Query
	if err := c.BindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiInternalError(err))
		return
	}

	signalChannel := fmt.Sprintf("/security-groups/org=%s", orgId.String())
	defaultOrderBy := "id"
	if v := c.Query("watch"); v == "true" {
		query.Sort = ""
		defaultOrderBy = "revision"
	}

	scopes := []func(*gorm.DB) *gorm.DB{
		func(db *gorm.DB) *gorm.DB {
			return db.Where("organization_id = ?", orgId)
		},
		FilterAndPaginateWithQuery(&models.SecurityGroup{}, c, query, defaultOrderBy),
	}

	api.sendListOrWatch(c, ctx, signalChannel, "revision", scopes, func(db *gorm.DB) (WatchableList, error) {
		var items securityGroupList
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
	if res := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		First(&org, "id = ?", orgId); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	var securityGroup models.SecurityGroup
	result := api.db.WithContext(ctx).
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
// @Failure		 401  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure		 429  {object}  models.BaseError
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
		if res := api.db.WithContext(ctx).
			Scopes(api.OrganizationIsOwnedByCurrentUser(c)).
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
		result := tx.Scopes(api.OrganizationIsOwnedByCurrentUser(c)).
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

	// TODO: signal bus notify of SecGroup deletes
	//api.signalBus.Notify

	c.JSON(http.StatusOK, sg)
}

// UpdateSecurityGroup updates a Security Group
// @Summary      Update Security Group
// @Description  Updates a Security Group by ID
// @Id  		 UpdateSecurityGroup
// @Tags         SecurityGroup
// @Accepts		 json
// @Produce      json
// @Param        organization_id   path      string  true "Organization ID"
// @Param        security_group_id   path      string  true "Security Group ID"
// @Param		 update body models.UpdateSecurityGroup true "Security Group Update"
// @Success      200  {object}  models.SecurityGroup
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
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
	var securityGroup models.SecurityGroup

	err = api.transaction(ctx, func(tx *gorm.DB) error {
		var org models.Organization
		if res := tx.WithContext(ctx).
			Scopes(api.OrganizationIsOwnedByCurrentUser(c)).
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

		if res := tx.Save(&securityGroup); res.Error != nil {
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
