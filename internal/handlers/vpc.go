package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
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

	var request models.AddVPC
	// Call BindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	if !request.PrivateCidr {
		if request.Ipv4Cidr == "" {
			request.Ipv4Cidr = defaultIPAMv4Cidr
		} else if request.Ipv4Cidr != defaultIPAMv4Cidr {
			c.JSON(http.StatusBadRequest, models.NewFieldValidationError("cidr", fmt.Sprintf("must be '%s' or not set when private_cidr is not enabled", defaultIPAMv4Cidr)))
			return
		}
		if request.Ipv6Cidr == "" {
			request.Ipv6Cidr = defaultIPAMv6Cidr
		} else if request.Ipv6Cidr != defaultIPAMv6Cidr {
			c.JSON(http.StatusBadRequest, models.NewFieldValidationError("cidr_v6", fmt.Sprintf("must be '%s' or not set when private_cidr is not enabled", defaultIPAMv6Cidr)))
			return
		}
	}

	if request.Ipv4Cidr == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("cidr"))
		return
	}
	if request.Ipv6Cidr == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("cidr_v6"))
		return
	}
	if request.OrganizationID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("organization_id"))
		return
	}

	var vpc models.VPC
	err := api.transaction(ctx, func(tx *gorm.DB) error {

		var org models.Organization
		if res := api.OrganizationIsReadableByCurrentUser(c, tx).
			First(&org, "id = ?", request.OrganizationID.String()); res.Error != nil {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("organization"))
		}

		vpc = models.VPC{
			OrganizationID: request.OrganizationID,
			Description:    request.Description,
			PrivateCidr:    request.PrivateCidr,
			Ipv4Cidr:       request.Ipv4Cidr,
			Ipv6Cidr:       request.Ipv6Cidr,
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

		if err := api.ipam.AssignCIDR(ctx, ipamNamespace, request.Ipv4Cidr); err != nil {
			api.logger.Error("Failed to assign IPv4 prefix: ", err)
			return err
		}

		if err := api.ipam.AssignCIDR(ctx, ipamNamespace, request.Ipv6Cidr); err != nil {
			api.logger.Error("Failed to assign IPv6 prefix: ", err)
			return err
		}

		// Create a default security group for the organization
		sg, err := api.createDefaultSecurityGroup(ctx, tx, vpc.ID)
		if err != nil {
			api.logger.Error("Failed to create default security group for VPC: ", err)
			return err
		}

		if err := api.updateVpcSecGroupId(ctx, tx, sg.ID, vpc.ID); err != nil {
			return fmt.Errorf("failed to update the default security group with a VPC id: %w", err)
		}
		vpc.SecurityGroupId = sg.ID

		span.SetAttributes(attribute.String("id", vpc.ID.String()))
		api.logger.Infof("New vpc request [ %s ] ipam v4 [ %s ] ipam v6 [ %s ] request", vpc.ID.String(), vpc.Ipv4Cidr, vpc.Ipv6Cidr)
		return nil
	})

	if err != nil {
		var duplicate errDuplicateVPC
		var apiResponseError *ApiResponseError
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewApiError(err))
		} else if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
		} else if errors.As(err, &duplicate) {
			c.JSON(http.StatusConflict, models.NewConflictsError(duplicate.ID))
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	c.JSON(http.StatusCreated, vpc)
}

func (api *API) VPCIsReadableByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	if api.dialect == database.DialectSqlLite {
		return db.Where("organization_id in (SELECT organization_id FROM user_organizations where user_id=?)", userId)
	} else {
		return db.Where("organization_id::text in (SELECT organization_id::text FROM user_organizations where user_id=?)", userId)
	}
}

func (api *API) VPCIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	if api.dialect == database.DialectSqlLite {
		return db.Where("organization_id in (SELECT id FROM organizations where owner_id=?)", userId)
	} else {
		return db.Where("organization_id::text in (SELECT id::text FROM organizations where owner_id=?)", userId)
	}
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
	db = FilterAndPaginate(db, &models.VPC{}, c, "description")
	result := db.Find(&vpcs)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("vpc"))
		} else {
			api.SendInternalServerError(c, result.Error)
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
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("vpc"))
		return
	}
	var vpc models.VPC
	db := api.db.WithContext(ctx)
	result := api.VPCIsReadableByCurrentUser(c, db).
		First(&vpc, "id = ?", id.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("vpc"))
		} else {
			api.SendInternalServerError(c, result.Error)
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
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	var query Query
	if err := c.ShouldBindQuery(&query); err != nil {
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
			attribute.String("id", c.Param("id")),
		))
	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var vpc models.VPC
	db := api.db.WithContext(ctx)
	result := api.VPCIsOwnedByCurrentUser(c, db).
		First(&vpc, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("vpc"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	if vpc.ID == vpc.OrganizationID {
		c.JSON(http.StatusBadRequest, models.NewNotAllowedError("default vpc cannot be deleted"))
		return
	}

	var count int64
	result = db.Model(&models.Device{}).Where("vpc_id = ?", id).Count(&count)
	if result.Error != nil {
		api.SendInternalServerError(c, result.Error)
		return
	}

	if count > 0 {
		c.JSON(http.StatusBadRequest, models.NewNotAllowedError("vpc cannot be delete while devices are still attached"))
		return
	}

	result = db.Delete(&vpc)
	if result.Error != nil {
		api.SendInternalServerError(c, result.Error)
		return
	}

	ipamNamespace := defaultIPAMNamespace
	if vpc.PrivateCidr {
		ipamNamespace = vpc.ID
	}
	vpcCIDR := vpc.Ipv4Cidr
	vpcCIDRV6 := vpc.Ipv6Cidr
	if vpc.PrivateCidr {
		if err := api.ipam.ReleaseCIDR(c.Request.Context(), ipamNamespace, vpcCIDR); err != nil {
			api.SendInternalServerError(c, fmt.Errorf("failed to release ipam vpc prefix: %w", err))
			return
		}

		if err := api.ipam.ReleaseCIDR(c.Request.Context(), ipamNamespace, vpcCIDRV6); err != nil {
			api.SendInternalServerError(c, fmt.Errorf("failed to release ipam vpc prefix: %w", err))
			return
		}
	}

	c.JSON(http.StatusOK, vpc)
}

// UpdateVPC updates a VPC
// @Summary      Update VPCs
// @Description  Updates a vpc by ID
// @Id  		 UpdateVPC
// @Tags         VPC
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "VPC ID"
// @Param		 update body models.UpdateVPC true "VPC Update"
// @Success      200  {object}  models.VPC
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs/{id} [patch]
func (api *API) UpdateVPC(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateVPC", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var request models.UpdateVPC
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	var vpc models.VPC
	err = api.transaction(ctx, func(tx *gorm.DB) error {

		result := api.VPCIsOwnedByCurrentUser(c, tx).First(&vpc, "id = ?", id)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("vpc"))
		}

		if request.Description != nil {
			vpc.Description = *request.Description
		}

		if res := tx.Save(&vpc); res.Error != nil {
			return res.Error
		}
		return nil
	})

	if err != nil {
		var apiResponseError *ApiResponseError
		if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}
	c.JSON(http.StatusOK, vpc)
}
