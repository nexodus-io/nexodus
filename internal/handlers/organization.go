package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultOrganizationPrefix = "100.100.0.0/16"
)

// CreateOrganization creates a new Organization
// @Summary      Create an Organization
// @Description  Creates a named organization with the given CIDR
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        Organization  body     models.AddOrganization  true  "Add Organization"
// @Success      201  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /organizations [post]
func (api *API) CreateOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateOrganization")
	defer span.End()
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return
	}
	allowForTests := c.GetString("nexodus.testCreateOrganization")
	if !multiOrganizationEnabled && allowForTests != "true" {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-organization support is disabled"))
		return
	}
	userId := c.GetString(gin.AuthUserKey)

	var request models.AddOrganization
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}
	if request.IpCidr == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("ip_cidr"))
		return
	}
	if request.Name == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("name"))
		return
	}

	var org models.Organization
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		var user models.User
		if res := tx.First(&user, "id = ?", userId); res.Error != nil {
			return errUserNotFound
		}

		org = models.Organization{
			Name:        request.Name,
			Description: request.Description,
			IpCidr:      request.IpCidr,
			HubZone:     request.HubZone,
			Users:       []*models.User{&user},
		}
		if res := tx.Create(&org); res.Error != nil {
			return res.Error
		}

		// Create the organization in IPAM
		if err := api.ipam.CreateNamespace(ctx, org.ID); err != nil {
			return err
		}

		if err := api.ipam.AssignPrefix(ctx, org.ID, request.IpCidr); err != nil {
			return err
		}

		span.SetAttributes(attribute.String("id", org.ID.String()))
		api.logger.Debugf("New organization request [ %s ] and ipam [ %s ] request", org.Name, org.IpCidr)
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
	c.JSON(http.StatusCreated, org)
}

// ListOrganizations lists all Organizations
// @Summary      List Organizations
// @Description  Lists all Organizations
// @Tags         Organization
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []models.Organization
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure		 500  {object}  models.BaseError
// @Router       /organizations [get]
func (api *API) ListOrganizations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListOrganizations")
	defer span.End()
	var orgs []models.Organization
	result := api.db.WithContext(ctx).Preload("Devices").Preload("Users").Scopes(FilterAndPaginate(&models.Organization{}, c)).Find(&orgs)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
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
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Router       /organizations/{id} [get]
func (api *API) GetOrganizations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetOrganizations",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	k, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}
	var org models.Organization
	result := api.db.WithContext(ctx).Preload("Devices").Preload("Users").First(&org, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
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
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure		 500  {object}  models.BaseError
// @Router       /organizations/{id}/devices [get]
func (api *API) ListDevicesInOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListDevicesInOrganization")
	defer span.End()
	k, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}
	var org models.Organization
	res := api.db.WithContext(ctx).Preload("Devices").First(&org, "id = ?", k.String())
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(res.Error))
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
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure		 500  {object}  models.BaseError
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
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var organization models.Organization
	result := api.db.WithContext(ctx).First(&organization, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var device models.Device
	result = api.db.WithContext(ctx).First(&device, "id = ?", id.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("device"))
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
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
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
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return
	}
	if !multiOrganizationEnabled {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-organization support is disabled"))
		return
	}

	orgID, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var org models.Organization
	if res := api.db.WithContext(ctx).First(&org, "id = ?", orgID); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	if res := api.db.Select(clause.Associations).Delete(&org); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to delete the organization: %w", err)))
		return
	}

	orgCIDR := org.IpCidr

	if orgCIDR != "" {
		if err := api.ipam.ReleasePrefix(c.Request.Context(), org.ID, orgCIDR); err != nil {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to release ipam organization prefix: %w", err)))
			return
		}
	}
	c.JSON(http.StatusOK, org)
}
