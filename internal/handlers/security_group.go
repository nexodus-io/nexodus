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

// ListSecurityGroups lists all Security Groups
// @Summary      List Security Groups
// @Description  Lists all Security Groups
// @Id  		 ListSecurityGroups
// @Tags         SecurityGroup
// @Accepts		 json
// @Produce      json
// @Param        organization_id   path      string  true "Organization ID"
// @Success      200  {object}  []models.SecurityGroup
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/organizations/{organization_id}/security_groups [get]
func (api *API) ListSecurityGroups(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListSecurityGroups")
	defer span.End()

	securityGroups := make([]models.SecurityGroup, 0)

	result := api.db.WithContext(ctx).Find(&securityGroups)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching security groups from db"})
		return
	}
	c.JSON(http.StatusOK, securityGroups)
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
// @Router       /api/organizations/{organization_id}/security_groups/{id} [get]
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
	var securityGroup models.SecurityGroup
	result := api.db.WithContext(ctx).
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
	ctx, span := tracer.Start(c.Request.Context(), "CreateSecurityGroup")
	defer span.End()

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

	var org models.Organization
	if res := api.db.WithContext(ctx).
		//Scopes(api.OrganizationIsOwnedByCurrentUser(c)). // TODO: Scoping
		First(&org, "id = ?", request.OrganizationId); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	var sg models.SecurityGroup
	err := api.transaction(ctx, func(tx *gorm.DB) error {

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
	ctx, span := tracer.Start(c.Request.Context(), "DeleteSecurityGroup")
	defer span.End()
	secGroupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	sg := models.SecurityGroup{}
	if res := api.db.
		//Scopes(api.DeviceIsOwnedByCurrentUser(c)). TODO: add scope
		First(&sg, "id = ?", secGroupID); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("security_group"))
		} else {
			c.JSON(http.StatusBadRequest, models.NewApiInternalError(res.Error))
		}
		return
	}

	if res := api.db.WithContext(ctx).
		Delete(&sg, "id = ?", sg.Base.ID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.NewApiInternalError(res.Error))
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
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var request models.UpdateSecurityGroup

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	var securityGroup models.SecurityGroup
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		result := tx.
			First(&securityGroup, "id = ?", k)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return errSecurityGroupNotFound
		}

		if request.GroupName != "" {
			securityGroup.GroupName = request.GroupName
		}

		if request.GroupDescription != "" {
			securityGroup.GroupDescription = request.GroupDescription
		}

		if request.InboundRules != nil {
			securityGroup.InboundRules = request.InboundRules
		}

		if request.OutboundRules != nil {
			securityGroup.OutboundRules = request.OutboundRules
		}

		if res := tx.
			// TODO: Add revision
			//Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Save(&securityGroup); res.Error != nil {
			return res.Error
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errSecurityGroupNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("security_group"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
	}

	c.JSON(http.StatusOK, securityGroup)
}

// createDefaultSecurityGroup creates the default security group for the organization
func (api *API) createDefaultSecurityGroup(ctx context.Context, orgId string) (models.SecurityGroup, error) {

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
	res := api.db.WithContext(ctx).Create(&sg)
	if res.Error != nil {
		return models.SecurityGroup{}, fmt.Errorf("failed to create the default organization security group: %w", err)
	}

	return sg, nil
}

// createDefaultSecurityGroup creates the default security group for the organization
func (api *API) updateDefaultSecurityGroupOrgId(ctx context.Context, sgId string, orgIdUUID uuid.UUID) error {

	var sg models.SecurityGroup
	res := api.db.Unscoped().First(&sg, "id = ?", sgId)
	if res.Error != nil {
		return res.Error
	}

	return api.db.Unscoped().Model(&sg).Update("OrganizationId", orgIdUUID).Error
}
