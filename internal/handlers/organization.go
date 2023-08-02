package handlers

import (
	"errors"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var defaultIPAMNamespace = uuid.UUID{}

const (
	defaultIPAMv4Cidr = "100.64.0.0/10"
	defaultIPAMv6Cidr = "200::/64"
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations [post]
func (api *API) CreateOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateOrganization")
	defer span.End()
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		api.sendInternalServerError(c, err)
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

	if !request.PrivateCidr {
		if request.IpCidr == "" {
			request.IpCidr = defaultIPAMv4Cidr
		} else if request.IpCidr != defaultIPAMv4Cidr {
			c.JSON(http.StatusBadRequest, models.NewFieldValidationError("cidr", fmt.Sprintf("must be '%s' or not set when private_cidr is not enabled", defaultIPAMv4Cidr)))
			return
		}
		if request.IpCidrV6 == "" {
			request.IpCidrV6 = defaultIPAMv4Cidr
		} else if request.IpCidrV6 != defaultIPAMv6Cidr {
			c.JSON(http.StatusBadRequest, models.NewFieldValidationError("cidr_v6", fmt.Sprintf("must be '%s' or not set when private_cidr is not enabled", defaultIPAMv6Cidr)))
			return
		}
	}

	if request.IpCidr == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("cidr"))
		return
	}
	if request.IpCidrV6 == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("cidr_v6"))
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
			PrivateCidr: request.PrivateCidr,
			IpCidr:      request.IpCidr,
			IpCidrV6:    request.IpCidrV6,
			HubZone:     request.HubZone,
			Users:       []*models.User{&user},
		}

		if res := tx.Create(&org); res.Error != nil {
			if database.IsDuplicateError(res.Error) {
				return errDuplicateOrganization{ID: org.ID.String()}
			}
			api.logger.Error("Failed to create organization: ", res.Error)
			return res.Error
		}

		ipamNamespace := defaultIPAMNamespace
		if org.PrivateCidr {
			ipamNamespace = org.ID
		}
		if err := api.ipam.CreateNamespace(ctx, ipamNamespace); err != nil {
			api.logger.Error("Failed to create namespace: ", err)
			return err
		}

		if err := api.ipam.AssignPrefix(ctx, ipamNamespace, request.IpCidr); err != nil {
			api.logger.Error("Failed to assign IPv4 prefix: ", err)
			return err
		}

		if err := api.ipam.AssignPrefix(ctx, ipamNamespace, request.IpCidrV6); err != nil {
			api.logger.Error("Failed to assign IPv6 prefix: ", err)
			return err
		}

		// Create a default security group for the organization
		sg, err := api.createDefaultSecurityGroup(ctx, tx, org.ID.String())
		if err != nil {
			api.logger.Error("Failed to create default security group for organization: ", err)
			return err
		}

		// Update the org with the new security group id
		if err := api.updateOrganizationSecGroupId(ctx, tx, sg.ID, org.ID); err != nil {
			return fmt.Errorf("failed to update the default security group with an org id: %w", err)
		}
		org.SecurityGroupId = sg.ID

		span.SetAttributes(attribute.String("id", org.ID.String()))
		api.logger.Infof("New organization request [ %s ] ipam v4 [ %s ] ipam v6 [ %s ] request", org.Name, org.IpCidr, org.IpCidrV6)
		return nil
	})

	if err != nil {
		var duplicate errDuplicateOrganization
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewApiError(err))
		} else if errors.As(err, &duplicate) {
			c.JSON(http.StatusConflict, models.NewConflictsError(duplicate.ID))
		} else {
			api.sendInternalServerError(c, err)
		}
		return
	}

	c.JSON(http.StatusCreated, org)
}

func (api *API) OrganizationIsReadableByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := c.Value(gin.AuthUserKey).(string)

	// this could potentially be driven by rego output
	if api.dialect == database.DialectSqlLite {
		return db.Where("owner_id = ? OR id in (SELECT organization_id FROM user_organizations where user_id=?)", userId, userId)
	} else {
		return db.Where("owner_id = ? OR id::text in (SELECT organization_id::text FROM user_organizations where user_id=?)", userId, userId)
	}
}

func (api *API) OrganizationIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := c.Value(gin.AuthUserKey).(string)
	// this could potentially be driven by rego output
	return db.Where("owner_id = ?", userId)
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations [get]
func (api *API) ListOrganizations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListOrganizations")
	defer span.End()
	var orgs []models.Organization

	db := api.db.WithContext(ctx)
	db = api.OrganizationIsReadableByCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.Organization{}, c, "name")
	result := db.Find(&orgs)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.sendInternalServerError(c, result.Error)
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
	db := api.db.WithContext(ctx)
	result := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", k.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.sendInternalServerError(c, result.Error)
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations/{organization_id}/devices [get]
func (api *API) ListDevicesInOrganization(c *gin.Context) {

	ctx, span := tracer.Start(c.Request.Context(), "ListDevicesInOrganization")
	defer span.End()

	orgId, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}
	var org models.Organization
	db := api.db.WithContext(ctx)
	result := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", orgId.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	var query Query
	if err := c.BindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiError(err))
		return
	}

	tokenClaims, err2 := NxodusClaims(c, api.db.WithContext(ctx))
	if err2 != nil {
		c.JSON(err2.Status, err2.Body)
		return
	}

	api.sendList(c, ctx, func(db *gorm.DB) (fetchmgr.ResourceList, error) {
		db = db.Where("organization_id = ?", orgId.String())
		db = FilterAndPaginateWithQuery(db, &models.Device{}, c, query, "hostname")

		var items deviceList
		result := db.Find(&items)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}

		for i, device := range items {
			if hideDeviceBearerToken(device, tokenClaims) {
				items[i].BearerToken = ""
			}
		}
		return items, nil
	})

}

type deviceList []*models.Device

func (d deviceList) Item(i int) (any, uint64, gorm.DeletedAt) {
	item := d[i]
	return item, item.Revision, item.DeletedAt
}

func (d deviceList) Len() int {
	return len(d)
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
	db := api.db.WithContext(ctx)
	result := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&organization, "id = ?", orgId.String())
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var device models.Device
	result = db.
		Where("organization_id = ?", orgId.String()).
		First(&device, "id = ?", id.String())
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("device"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	tokenClaims, err2 := NxodusClaims(c, api.db.WithContext(ctx))
	if err2 != nil {
		c.JSON(err2.Status, err2.Body)
		return
	}

	// only show the device token when using the reg token that created the device.
	if hideDeviceBearerToken(&device, tokenClaims) {
		device.BearerToken = ""
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
	db := api.db.WithContext(ctx)
	result := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", k.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	var users []*models.User
	db = db.
		Joins("inner join user_organizations on user_organizations.user_id=users.id").
		Where("user_organizations.organization_id = ?", k.String())
	db = FilterAndPaginate(db, &models.User{}, c, "user_name")
	result = db.Find(&users)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		api.sendInternalServerError(c, result.Error)
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
		api.sendInternalServerError(c, err)
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
	db := api.db.WithContext(ctx)
	result := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", orgID)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	ipamNamespace := defaultIPAMNamespace
	if org.PrivateCidr {
		ipamNamespace = org.ID
	}

	type userOrgMapping struct {
		UserID         string
		OrganizationID uuid.UUID
	}
	var usersInOrg []userOrgMapping
	if res := db.Table("user_organizations").Select("user_id", "organization_id").Where("organization_id = ?", org.ID).Scan(&usersInOrg); res.Error != nil {
		api.sendInternalServerError(c, res.Error)
		return
	}

	if res := api.db.Select(clause.Associations).Delete(&org); res.Error != nil {
		api.sendInternalServerError(c, fmt.Errorf("failed to delete the organization: %w", err))
		return
	}

	orgCIDR := org.IpCidr
	orgCIDRV6 := org.IpCidrV6

	if org.PrivateCidr {
		if err := api.ipam.ReleasePrefix(c.Request.Context(), ipamNamespace, orgCIDR); err != nil {
			api.sendInternalServerError(c, fmt.Errorf("failed to release ipam organization prefix: %w", err))
			return
		}

		if err := api.ipam.ReleasePrefix(c.Request.Context(), ipamNamespace, orgCIDRV6); err != nil {
			api.sendInternalServerError(c, fmt.Errorf("failed to release ipam organization prefix: %w", err))
			return
		}
	}

	c.JSON(http.StatusOK, org)
}
