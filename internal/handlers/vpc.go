package handlers

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
)

var defaultIPAMNamespace = uuid.UUID{}

const (
	defaultIPAMv4Cidr = "100.64.0.0/10"
	defaultIPAMv6Cidr = "200::/64"
)

type errDuplicateVPC struct {
	ID string
}

func (e errDuplicateVPC) Error() string {
	return "vpc already exists"
}

// CreateVPC creates a new VPC
// @Summary      Create an VPC
// @Description  Creates a named vpc with the given CIDR
// @Id			 CreateVPC
// @Tags         VPC
// @Accept       json
// @Produce      json
// @Param        VPC  body     models.AddVPC  true  "Add VPC"
// @Success      201  {object}  models.VPC
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 405  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs [post]
func (api *API) CreateVPC(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateVPC")
	defer span.End()
	multiVPCEnabled, err := api.fflags.GetFlag("multi-vpc")
	if err != nil {
		api.sendInternalServerError(c, err)
		return
	}
	allowForTests := c.GetString("nexodus.testCreateVPC")
	if (!multiVPCEnabled && allowForTests != "true") || allowForTests == "false" {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-vpc support is disabled"))
		return
	}

	var request models.AddVPC
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
	if request.OrganizationID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("organization_id"))
		return
	}

	var vpc models.VPC
	err = api.transaction(ctx, func(tx *gorm.DB) error {

		var org models.Organization
		if res := api.OrganizationIsReadableByCurrentUser(c, tx).
			First(&org, "id = ?", request.OrganizationID.String()); res.Error != nil {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("organization"))
		}

		vpc = models.VPC{
			OrganizationID: request.OrganizationID,
			Description:    request.Description,
			PrivateCidr:    request.PrivateCidr,
			IpCidr:         request.IpCidr,
			IpCidrV6:       request.IpCidrV6,
			HubZone:        request.HubZone,
		}

		if res := tx.Create(&vpc); res.Error != nil {
			if database.IsDuplicateError(res.Error) {
				return errDuplicateVPC{ID: vpc.ID.String()}
			}
			api.logger.Error("Failed to create vpc: ", res.Error)
			return res.Error
		}

		ipamNamespace := defaultIPAMNamespace
		if vpc.PrivateCidr {
			ipamNamespace = vpc.ID
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

		span.SetAttributes(attribute.String("id", vpc.ID.String()))
		api.logger.Infof("New vpc request [ %s ] ipam v4 [ %s ] ipam v6 [ %s ] request", vpc.ID.String(), vpc.IpCidr, vpc.IpCidrV6)
		return nil
	})

	if err != nil {
		var duplicate errDuplicateVPC
		var apiResponseError ApiResponseError
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewApiError(err))
		} else if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
		} else if errors.As(err, &duplicate) {
			c.JSON(http.StatusConflict, models.NewConflictsError(duplicate.ID))
		} else {
			api.sendInternalServerError(c, err)
		}
		return
	}

	c.JSON(http.StatusCreated, vpc)
}

func (api *API) VPCIsReadableByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := c.Value(gin.AuthUserKey).(string)
	if api.dialect == database.DialectSqlLite {
		return db.Where("organization_id in (SELECT organization_id FROM user_organizations where user_id=?)", userId, userId)
	} else {
		return db.Where("organization_id::text in (SELECT organization_id::text FROM user_organizations where user_id=?)", userId, userId)
	}
}

func (api *API) VPCIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := c.Value(gin.AuthUserKey).(string)
	// this could potentially be driven by rego output
	return db.Where("owner_id = ?", userId)
}

// ListVPCs lists all VPCs
// @Summary      List VPCs
// @Description  Lists all VPCs
// @Id 			 ListVPCs
// @Tags         VPC
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.VPC
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs [get]
func (api *API) ListVPCs(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListVPCs")
	defer span.End()
	var vpcs []models.VPC

	db := api.db.WithContext(ctx)
	db = api.VPCIsReadableByCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.VPC{}, c, "name")
	result := db.Find(&vpcs)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("vpc"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	c.JSON(http.StatusOK, vpcs)
}

// GetVPC gets a specific VPC
// @Summary      Get VPCs
// @Description  Gets a VPC by VPC ID
// @Id 			 GetVPC
// @Tags         VPC
// @Accept       json
// @Produce      json
// @Param		 id   path      string true "VPC ID"
// @Success      200  {object}  models.VPC
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs/{id} [get]
func (api *API) GetVPC(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetVPCs",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	k, err := uuid.Parse(c.Param("vpc"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("vpc"))
		return
	}
	var vpc models.VPC
	db := api.db.WithContext(ctx)
	result := api.VPCIsReadableByCurrentUser(c, db).
		First(&vpc, "id = ?", k.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("vpc"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	c.JSON(http.StatusOK, vpc)
}

// ListDevicesInVPC lists all devices in an VPC
// @Summary      List Devices
// @Description  Lists all devices for this VPC
// @Id           ListDevicesInVPC
// @Tags         VPC
// @Accept       json
// @Produce      json
// @Param		 gt_revision     query  uint64   false "greater than revision"
// @Param		 id              path   string true "VPC ID"
// @Success      200  {object}  []models.Device
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs/{id}/devices [get]
func (api *API) ListDevicesInVPC(c *gin.Context) {

	ctx, span := tracer.Start(c.Request.Context(), "ListDevicesInVPC",
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
		db = db.Where("vpc_id = ?", vpcId.String())
		db = FilterAndPaginateWithQuery(db, &models.Device{}, c, query, "hostname")

		var items deviceList
		result := db.Find(&items)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}

		for i := range items {
			hideDeviceBearerToken(items[i], tokenClaims)
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

// GetDeviceInVPC gets a device in a VPC
// @Summary      Get Device
// @Description  Gets a device in a vpc by ID
// @Id 			 GetDeviceInVPC
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param		 vpc_id path   string true "VPC ID"
// @Param		 device_id path   string true "Device ID"
// @Success      200  {object}  models.Device
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs/{vpc_id}/devices/{device_id} [get]
func (api *API) GetDeviceInVPC(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetDeviceInVPC",
		trace.WithAttributes(
			attribute.String("vpc", c.Param("vpc")),
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	vpcId, err := uuid.Parse(c.Param("vpc"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("vpc"))
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
		Where("vpc_id = ?", vpcId.String()).
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
	hideDeviceBearerToken(&device, tokenClaims)

	c.JSON(http.StatusOK, device)
}

// DeleteVPC handles deleting an existing vpc and associated ipam prefix
// @Summary      Delete VPC
// @Description  Deletes an existing vpc and associated IPAM prefix
// @Id 			 DeleteVPC
// @Tags         VPC
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "VPC ID"
// @Success      204  {object}  models.VPC
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs/{id} [delete]
func (api *API) DeleteVPC(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteVPC",
		trace.WithAttributes(
			attribute.String("vpc", c.Param("vpc")),
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	multiVPCEnabled, err := api.fflags.GetFlag("multi-vpc")
	if err != nil {
		api.sendInternalServerError(c, err)
		return
	}
	if !multiVPCEnabled {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-vpc support is disabled"))
		return
	}

	vpcID, err := uuid.Parse(c.Param("vpc"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("vpc"))
		return
	}

	var vpc models.VPC
	db := api.db.WithContext(ctx)
	result := api.VPCIsReadableByCurrentUser(c, db).
		First(&vpc, "id = ?", vpcID)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("vpc"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	ipamNamespace := defaultIPAMNamespace
	if vpc.PrivateCidr {
		ipamNamespace = vpc.ID
	}

	type userOrgMapping struct {
		UserID string
		VPCID  uuid.UUID
	}
	var usersInOrg []userOrgMapping
	if res := db.Table("user_vpcs").Select("user_id", "vpc_id").Where("vpc_id = ?", vpc.ID).Scan(&usersInOrg); res.Error != nil {
		api.sendInternalServerError(c, res.Error)
		return
	}

	if res := api.db.Select(clause.Associations).Delete(&vpc); res.Error != nil {
		api.sendInternalServerError(c, fmt.Errorf("failed to delete the vpc: %w", err))
		return
	}

	vpcCIDR := vpc.IpCidr
	vpcCIDRV6 := vpc.IpCidrV6

	if vpc.PrivateCidr {
		if err := api.ipam.ReleasePrefix(c.Request.Context(), ipamNamespace, vpcCIDR); err != nil {
			api.sendInternalServerError(c, fmt.Errorf("failed to release ipam vpc prefix: %w", err))
			return
		}

		if err := api.ipam.ReleasePrefix(c.Request.Context(), ipamNamespace, vpcCIDRV6); err != nil {
			api.sendInternalServerError(c, fmt.Errorf("failed to release ipam vpc prefix: %w", err))
			return
		}
	}

	c.JSON(http.StatusOK, vpc)
}
