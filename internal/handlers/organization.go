package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultOrganizationPrefixIPv4 = "100.100.0.0/16"
	defaultOrganizationPrefixIPv6 = "200::/64"
)

type errDuplicateOrganization struct {
	ID string
}

func (e errDuplicateOrganization) Error() string {
	return "organization already exists"
}

// CreateOrganization creates a new Organization
// @Summary      Create an Organization
// @Description  Creates a named organization with the given CIDR
// @Id			 CreateOrganization
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        Organization  body     models.AddOrganization  true  "Add Organization"
// @Success      201  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 405  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /api/organizations [post]
func (api *API) CreateOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateOrganization")
	defer span.End()
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return
	}
	allowForTests := c.GetString("nexodus.testCreateOrganization")
	if (!multiOrganizationEnabled && allowForTests != "true") || allowForTests == "false" {
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
	if request.IpCidrV6 == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("ip_cidr_v6"))
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
			OwnerID:     userId,
			Description: request.Description,
			IpCidr:      request.IpCidr,
			IpCidrV6:    request.IpCidrV6,
			HubZone:     request.HubZone,
			Users:       []*models.User{&user},
		}

		if res := tx.Create(&org); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
				return errDuplicateOrganization{ID: org.ID.String()}
			}
			return res.Error
		}

		// Create the organization in IPAM
		if err := api.ipam.CreateNamespace(ctx, org.ID); err != nil {
			return err
		}

		if err := api.ipam.AssignPrefix(ctx, org.ID, request.IpCidr); err != nil {
			return err
		}

		if err := api.ipam.AssignPrefix(ctx, org.ID, request.IpCidrV6); err != nil {
			return err
		}

		// Create a default security group for the organization
		sg, err := api.createDefaultSecurityGroup(ctx, tx, org.ID.String())
		if err != nil {
			return err
		}

		// Update the org with the new security group id
		if err := api.updateOrganizationSecGroupId(ctx, tx, sg.ID, org.ID); err != nil {
			return fmt.Errorf("failed to update the default security group with an org id: %v", err.Error())
		}

		span.SetAttributes(attribute.String("id", org.ID.String()))
		api.logger.Infof("New organization request [ %s ] ipam v4 [ %s ] ipam v6 [ %s ] request", org.Name, org.IpCidr, org.IpCidrV6)
		return nil
	})

	if err != nil {
		var duplicate errDuplicateOrganization
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewApiInternalError(err))
		} else if errors.As(err, &duplicate) {
			c.JSON(http.StatusConflict, models.NewConflictsError(duplicate.ID))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
	}

	c.JSON(http.StatusCreated, org)
}

func (api *API) OrganizationIsReadableByCurrentUser(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)

		// this could potentially be driven by rego output
		if api.dialect == database.DialectSqlLite {
			return db.Where("owner_id = ? OR id in (SELECT organization_id FROM user_organizations where user_id=?)", userId, userId)
		} else {
			return db.Where("owner_id = ? OR id::text in (SELECT organization_id::text FROM user_organizations where user_id=?)", userId, userId)
		}
	}
}

func (api *API) OrganizationIsOwnedByCurrentUser(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)
		// this could potentially be driven by rego output
		return db.Where("owner_id = ?", userId)
	}
}

// ListOrganizations lists all Organizations
// @Summary      List Organizations
// @Description  Lists all Organizations
// @Id 			 ListOrganizations
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.Organization
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure		 500  {object}  models.BaseError
// @Router       /api/organizations [get]
func (api *API) ListOrganizations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListOrganizations")
	defer span.End()
	var orgs []models.Organization
	result := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		Scopes(FilterAndPaginate(&models.Organization{}, c, "name")).
		Find(&orgs)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		}
		return
	}

	c.JSON(http.StatusOK, orgs)
}

// GetOrganizations gets a specific Organization
// @Summary      Get Organizations
// @Description  Gets a Organization by Organization ID
// @Id 			 GetOrganizations
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param		 id   path      string true "Organization ID"
// @Success      200  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Router       /api/organizations/{id} [get]
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
	result := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		First(&org, "id = ?", k.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		}
		return
	}

	c.JSON(http.StatusOK, org)
}

// ListDevicesInOrganization lists all devices in an Organization
// @Summary      List Devices
// @Description  Lists all devices for this Organization
// @Id           ListDevicesInOrganization
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param		 gt_revision     query  uint64 false "greater than revision"
// @Param		 organization_id path   string true "Organization ID"
// @Success      200  {object}  []models.Device
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure		 500  {object}  models.BaseError
// @Router       /api/organizations/{organization_id}/devices [get]
func (api *API) ListDevicesInOrganization(c *gin.Context) {

	ctx, span := tracer.Start(c.Request.Context(), "ListDevicesInOrganization")
	defer span.End()

	k, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}
	var org models.Organization
	result := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		First(&org, "id = ?", k.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		}
		return
	}

	var query Query
	if err := c.BindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiInternalError(err))
		return
	}
	defaultOrderBy := "hostname"

	gtRevision := uint64(0)
	if v := c.Query("gt_revision"); v != "" {
		gtRevision, _ = strconv.ParseUint(v, 10, 0)
	}

	includeDeleted := false

	getList := func() ([]*models.Device, error) {
		var devices []*models.Device

		db := api.db.WithContext(ctx)
		if includeDeleted {
			db = db.Unscoped()
		}
		db = db.Scopes(FilterAndPaginateWithQuery(&models.Device{}, c, query, defaultOrderBy)).
			Where("organization_id = ?", k.String())

		if gtRevision != 0 {
			db = db.Where("revision > ?", gtRevision)
		}

		result = db.Find(&devices)

		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
		return devices, nil
	}

	if v := c.Query("watch"); v == "true" {
		query.Sort = ""
		defaultOrderBy = "revision"
		includeDeleted = true
		sub := api.signalBus.Subscribe(fmt.Sprintf("/devices/org=%s", k.String()))
		defer sub.Close()

		idx := 1
		var list []*models.Device
		bookmarkSent := false

		c.Header("Content-Type", "application/json;stream=watch")
		c.Status(http.StatusOK)
		stream(c, func() models.WatchEvent {
			// This function blocks until there is an event to return...
			for {
				if err != nil {
					return models.WatchEvent{
						Type:  "error",
						Value: err.Error(),
					}
				}
				if idx < len(list) {
					result := list[idx]
					gtRevision = result.Revision
					idx += 1

					if result.DeletedAt.Valid {
						return models.WatchEvent{
							Type:  "delete",
							Value: result,
						}
					} else {
						return models.WatchEvent{
							Type:  "change",
							Value: result,
						}
					}
				} else {

					// get the next list...
					list, err = getList()
					if err != nil {
						return models.WatchEvent{
							Type:  "error",
							Value: err.Error(),
						}
					}
					idx = 0

					// did we run out of items to send?
					if len(list) == 0 {

						// bookmark idea taken from: https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks
						if !bookmarkSent {
							bookmarkSent = true
							return models.WatchEvent{
								Type: "bookmark",
							}
						}

						// Wait for some items to come into the list
						if waitForCancelOrTimeoutOrNotification(ctx, 30*time.Second, sub) {
							// ctx was canceled... likely due to the http connection being closed by
							// the client.  Signal the event stream is done.
							return models.WatchEvent{
								Type: "close",
							}
						}
					}
				}
			}
		})

	} else {
		devices, err := getList()
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
			return
		}

		// For pagination
		c.Header("Access-Control-Expose-Headers", TotalCountHeader)
		c.Header(TotalCountHeader, strconv.Itoa(len(devices)))
		c.JSON(http.StatusOK, devices)
	}

}

// GetDeviceInOrganization gets a device in a Organization
// @Summary      Get Device
// @Description  Gets a device in a organization by ID
// @Id 			 GetDeviceInOrganization
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param		 organization_id path   string true "Organization ID"
// @Param		 device_id path   string true "Device ID"
// @Success      200  {object}  models.Device
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure		 500  {object}  models.BaseError
// @Router       /api/organizations/{organization_id}/devices/{device_id} [get]
func (api *API) GetDeviceInOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetDeviceInOrganization",
		trace.WithAttributes(
			attribute.String("organization", c.Param("organization")),
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	orgId, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var organization models.Organization
	result := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		First(&organization, "id = ?", orgId.String())
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		}
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var device models.Device
	result = api.db.WithContext(ctx).
		Where("organization_id = ?", orgId.String()).
		First(&device, "id = ?", id.String())
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("device"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		}
		return
	}
	c.JSON(http.StatusOK, device)
}

// ListUsersInOrganization lists all users in a Organization
// @Summary      List Users
// @Description  Lists all users for this Organization
// @Id           ListUsersInOrganization
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param		 id   path       string true "Organization ID"
// @Success      200  {object}  []models.User
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure		 500  {object}  models.BaseError
// @Router       /api/organizations/{id}/devices [get]
func (api *API) ListUsersInOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListUsersInOrganization")
	defer span.End()
	k, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}
	var org models.Organization
	result := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		First(&org, "id = ?", k.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		}
		return
	}

	var users []*models.User
	result = api.db.WithContext(ctx).
		Joins("inner join user_organizations on user_organizations.user_id=users.id").
		Where("user_organizations.organization_id = ?", k.String()).
		Scopes(FilterAndPaginate(&models.User{}, c, "user_name")).
		Find(&users)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		return
	}

	// For pagination
	c.Header("Access-Control-Expose-Headers", TotalCountHeader)
	c.Header(TotalCountHeader, strconv.Itoa(len(users)))
	c.JSON(http.StatusOK, users)
}

// DeleteOrganization handles deleting an existing organization and associated ipam prefix
// @Summary      Delete Organization
// @Description  Deletes an existing organization and associated IPAM prefix
// @Id 			 DeleteOrganization
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "Organization ID"
// @Success      204  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /api/organizations/{id} [delete]
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
	result := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		First(&org, "id = ?", orgID)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		}
		return
	}

	type userOrgMapping struct {
		UserID         string
		OrganizationID uuid.UUID
	}
	var usersInOrg []userOrgMapping
	if res := api.db.WithContext(ctx).Table("user_organizations").Select("user_id", "organization_id").Where("organization_id = ?", org.ID).Scan(usersInOrg); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(res.Error))
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
