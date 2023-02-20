package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gorm.io/gorm"
)

const (
	defaultOrganizationPrefix = "10.200.1.0/20"
)

// CreateOrganization creates a new Organization
// @Summary      Create a Organization
// @Description  Creates a named organization with the given CIDR
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        Organization  body     models.AddOrganization  true  "Add Organization"
// @Success      201  {object}  models.Organization
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure		 405  {object}  models.ApiError
// @Failure      500  {object}  models.ApiError
// @Router       /organizations [post]
func (api *API) CreateOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateOrganization")
	defer span.End()
	tx := api.db.Begin().WithContext(ctx)
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: err.Error()})
		return
	}
	allowForTests := c.GetString("_apex.testCreateOrganization")
	if !multiOrganizationEnabled && allowForTests != "true" {
		c.JSON(http.StatusMethodNotAllowed, models.ApiError{Error: "multi-organization support is disabled"})
		return
	}

	userId := c.GetString(gin.AuthUserKey)
	var user models.User
	if res := tx.First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "user not found"})
		return
	}

	var request models.AddOrganization
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: err.Error()})
		return
	}
	if request.IpCidr == "" {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "the organization request did not contain a required CIDR prefix"})
		return
	}
	if request.Name == "" {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "the organization request did not contain a required name"})
		return
	}

	newOrganization := models.Organization{
		Name:        request.Name,
		Description: request.Description,
		IpCidr:      request.IpCidr,
		HubZone:     request.HubZone,
		Users:       []*models.User{&user},
	}
	if res := tx.Create(&newOrganization); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "unable to create organization"})
		return
	}

	// Create the organization in IPAM
	if err := api.ipam.CreateNamespace(ctx, newOrganization.ID.String()); err != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "unable to create ipam namespace"})
		return
	}

	if err := api.ipam.AssignPrefix(ctx, newOrganization.ID.String(), request.IpCidr); err != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: err.Error()})
		return
	}

	span.SetAttributes(attribute.String("id", newOrganization.ID.String()))
	api.logger.Debugf("New organization request [ %s ] and ipam [ %s ] request", newOrganization.Name, newOrganization.IpCidr)
	if err := tx.Commit(); err.Error != nil {
		tx.Rollback()
		api.Logger(ctx).Error(err.Error)
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}
	c.JSON(http.StatusCreated, newOrganization)
}

// ListOrganizations lists all Organizations
// @Summary      List Organizations
// @Description  Lists all Organizations
// @Tags         Organization
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []models.Organization
// @Failure		 401  {object}  models.ApiError
// @Failure		 500  {object}  models.ApiError
// @Router       /organizations [get]
func (api *API) ListOrganizations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListOrganizations")
	defer span.End()
	tx := api.db.Begin().WithContext(ctx)
	var orgs []models.Organization
	result := tx.Preload("Devices").Preload("Users").Scopes(FilterAndPaginate(&models.Organization{}, c)).Find(&orgs)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "error fetching Organizations from db"})
		return
	}
	if err := tx.Commit(); err.Error != nil {
		tx.Rollback()
		api.Logger(ctx).Error(err.Error)
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}
	c.JSON(http.StatusOK, orgs)
}

// GetOrganizations gets a specific Organization
// @Summary      Get Organizations
// @Description  Gets a Organization by Organization ID
// @Tags         Organization
// @Accepts		 json
// @Produce      json
// @Param		 id   path      string true "Organization ID"
// @Success      200  {object}  models.Organization
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure      404  {object}  models.ApiError
// @Router       /organizations/{id} [get]
func (api *API) GetOrganizations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetOrganizations",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	k, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "Organization id is not a valid UUID"})
		return
	}
	var org models.Organization
	result := api.db.WithContext(ctx).Preload("Devices").Preload("Users").First(&org, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "Organization not found"})
		return
	}
	c.JSON(http.StatusOK, org)
}

// ListDevicesInOrganization lists all devices in a Organization
// @Summary      List Devices
// @Description  Lists all devices for this Organization
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param		 id   path       string true "Organization ID"
// @Success      200  {object}  []models.Device
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure		 500  {object}  models.ApiError
// @Router       /organizations/{id}/devices [get]
func (api *API) ListDevicesInOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListDevicesInOrganization")
	defer span.End()
	k, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "organization id is not a valid UUID"})
		return
	}
	var org models.Organization
	res := api.db.WithContext(ctx).Preload("Devices").First(&org, "id = ?", k.String())
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "error fetching devices from db"})
		return
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", TotalCountHeader)
	c.Header(TotalCountHeader, strconv.Itoa(len(org.Devices)))
	c.JSON(http.StatusOK, org.Devices)
}

// GetDeviceInOrganization gets a device in a Organization
// @Summary      Get Device
// @Description  Gets a device in a organization by ID
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param		 organization_id path   string true "Organization ID"
// @Param		 device_id path   string true "Device ID"
// @Success      200  {object}  []models.Device
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure      404  {object}  models.ApiError
// @Failure		 500  {object}  models.ApiError
// @Router       /organizations/{organization_id}/devices/{device_id} [get]
func (api *API) GetDeviceInOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetDeviceInOrganization",
		trace.WithAttributes(
			attribute.String("organization", c.Param("organization")),
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	k, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "Organization id is not a valid UUID"})
		return
	}

	var organization models.Organization
	result := api.db.WithContext(ctx).First(&organization, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "organization not found"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "organization id is not a valid UUID"})
		return
	}
	var device models.Device
	result = api.db.WithContext(ctx).First(&device, "id = ?", id.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "device not found"})
		return
	}
	c.JSON(http.StatusOK, device)
}

// DeleteOrganization handles deleting an existing organization and associated ipam prefix
// @Summary      Delete Organization
// @Description  Deletes an existing organization and associated IPAM prefix
// @Tags         Organizations
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Organization ID"
// @Success      204  {object}  models.Organization
// @Failure      400  {object}  models.ApiError
// @Failure      405  {object}  models.ApiError
// @Failure      500  {object}  models.ApiError
// @Router       /organizations/{id} [delete]
func (api *API) DeleteOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteOrganization",
		trace.WithAttributes(
			attribute.String("organization", c.Param("organization")),
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: err.Error()})
		return
	}
	if !multiOrganizationEnabled {
		c.JSON(http.StatusMethodNotAllowed, models.ApiError{Error: "multi-organization support is disabled"})
		return
	}

	orgID, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "organization id is not valid"})
		return
	}

	var org models.Organization
	if res := api.db.WithContext(ctx).First(&org, "id = ?", orgID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: res.Error.Error()})
		return
	}

	orgCIDR := org.IpCidr

	if res := api.db.WithContext(ctx).Delete(&org, "id = ?", orgID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: res.Error.Error()})
		return
	}

	if orgCIDR != "" {
		if err := api.ipam.ReleasePrefix(c.Request.Context(), org.ID.String(), orgCIDR); err != nil {
			c.JSON(http.StatusInternalServerError, models.ApiError{Error: fmt.Sprintf("failed to release ipam organization prefix: %v", err)})
		}
	}
	c.JSON(http.StatusOK, org)
}
