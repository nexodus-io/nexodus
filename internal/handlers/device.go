package handlers

import (
	"errors"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/database"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

var (
	errUserOrOrgNotFound = errors.New("user or organization not found")
	errUserNotFound      = errors.New("user not found")
	errDeviceNotFound    = errors.New("device not found")
	errOrgNotFound       = errors.New("organization not found")
)

type errDuplicateDevice struct {
	ID string
}

func (e errDuplicateDevice) Error() string {
	return "device already exists"
}

// ListDevices lists all devices
// @Summary      List Devices
// @Description  Lists all devices
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []models.Device
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /devices [get]
func (api *API) ListDevices(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListDevices")
	defer span.End()
	devices := make([]models.Device, 0)

	result := api.db.WithContext(ctx).Scopes(
		api.DeviceIsVisibleToCurrentUser(c),
		FilterAndPaginate(&models.Device{}, c),
	).Find(&devices)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	c.JSON(http.StatusOK, devices)
}

func (api *API) DeviceIsVisibleToCurrentUser(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)
		// this could potentially be driven by rego output
		if api.dialect == database.DialectSqlLite {
			return db.Where("user_id = ? OR organization_id in (SELECT organization_id FROM user_organizations where user_id=?)", userId, userId)
		} else {
			return db.Where("user_id = ? OR organization_id::text in (SELECT organization_id::text FROM user_organizations where user_id=?)", userId, userId)
		}
	}
}

// GetDevice gets a device by ID
// @Summary      Get Devices
// @Description  Gets a device by ID
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      200  {object}  models.Device
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /devices/{id} [get]
func (api *API) GetDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetDevice", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var device models.Device
	result := api.db.WithContext(ctx).
		Scopes(api.DeviceIsVisibleToCurrentUser(c)).
		First(&device, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, device)
}

// UpdateDevice updates a Device
// @Summary      Update Devices
// @Description  Updates a device by ID
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Param		 update body models.UpdateDevice true "Device Update"
// @Success      200  {object}  models.Device
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /devices/{id} [patch]
func (api *API) UpdateDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateDevice", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var request models.UpdateDevice

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	var device models.Device
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		result := tx.
			Scopes(api.DeviceIsVisibleToCurrentUser(c)).
			First(&device, "id = ?", k)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return errDeviceNotFound
		}

		if request.EndpointLocalAddressIPv4 != "" {
			device.EndpointLocalAddressIPv4 = request.EndpointLocalAddressIPv4
		}

		if request.Hostname != "" {
			device.Hostname = request.Hostname
		}

		if request.LocalIP != "" {
			device.LocalIP = request.LocalIP
		}

		if request.OrganizationID != uuid.Nil {
			device.OrganizationID = request.OrganizationID
		}

		if request.ReflexiveIPv4 != "" {
			device.ReflexiveIPv4 = request.ReflexiveIPv4
		}

		if request.SymmetricNat != nil {
			device.SymmetricNat = *request.SymmetricNat
		}

		if request.ChildPrefix != nil {
			prefixAllocated := make(map[string]struct{})
			for _, prefix := range device.ChildPrefix {
				prefixAllocated[prefix] = struct{}{}
			}
			for _, prefix := range request.ChildPrefix {
				// lookup miss of prefix means we need to release it
				if _, ok := prefixAllocated[prefix]; ok {
					if err := api.ipam.ReleasePrefix(ctx, device.OrganizationID, prefix); err != nil {
						return err
					}
				} else {
					// otherwise we need to allocate it
					if err := api.ipam.AssignPrefix(ctx, device.OrganizationID, prefix); err != nil {
						return err
					}
				}
			}
			device.ChildPrefix = request.ChildPrefix
		}

		if res := tx.Save(&device); res.Error != nil {
			return res.Error
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, errDeviceNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("device"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
	}
	c.JSON(http.StatusOK, device)
}

// CreateDevice handles adding a new device
// @Summary      Add Devices
// @Description  Adds a new device
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        device  body   models.AddDevice  true "Add Device"
// @Success      201  {object}  models.Device
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure      409  {object}  models.Device
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /devices [post]
func (api *API) CreateDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateDevice")
	defer span.End()
	var request models.AddDevice
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	if request.PublicKey == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("public_key"))
		return
	}

	userId := c.GetString(gin.AuthUserKey)
	var device models.Device

	err := api.transaction(ctx, func(tx *gorm.DB) error {

		var org models.Organization
		if res := tx.Model(&org).
			Joins("inner join user_organizations on user_organizations.organization_id=organizations.id").
			Where("user_organizations.user_id=? AND organizations.id=?", userId, request.OrganizationID).
			First(&org); res.Error != nil {
			return errUserOrOrgNotFound
		}

		res := tx.Where("public_key = ?", request.PublicKey).First(&device)
		if res.Error == nil {
			return errDuplicateDevice{ID: device.ID.String()}
		}
		if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return res.Error
		}

		ipamPrefix := org.IpCidr
		var relay bool
		// determine if the node joining is a relay node
		if request.Relay {
			relay = true
		}

		var ipamIP string
		// If this was a static address request
		// TODO: handle a user requesting an IP not in the IPAM prefix
		if request.TunnelIP != "" {
			var err error
			ipamIP, err = api.ipam.AssignSpecificTunnelIP(ctx, org.ID, ipamPrefix, request.TunnelIP)
			if err != nil {
				return fmt.Errorf("failed to request specific ipam address: %w", err)
			}
		} else {
			var err error
			ipamIP, err = api.ipam.AssignFromPool(ctx, org.ID, ipamPrefix)
			if err != nil {
				return fmt.Errorf("failed to request ipam address: %w", err)
			}
		}
		// allocate a child prefix if requested
		for _, prefix := range request.ChildPrefix {
			if err := api.ipam.AssignPrefix(ctx, org.ID, prefix); err != nil {
				return fmt.Errorf("failed to assign child prefix: %w", err)
			}
		}

		// append a /32 to the IPAM assignment unless it is a relay prefix
		hostPrefix := ipamIP
		if net.ParseIP(ipamIP) != nil && !relay {
			hostPrefix = fmt.Sprintf("%s/32", ipamIP)
		}

		var allowedIPs []string
		allowedIPs = append(allowedIPs, hostPrefix)

		device = models.Device{
			UserID:                   userId,
			OrganizationID:           org.ID,
			PublicKey:                request.PublicKey,
			LocalIP:                  request.LocalIP,
			AllowedIPs:               allowedIPs,
			TunnelIP:                 ipamIP,
			ChildPrefix:              request.ChildPrefix,
			Relay:                    request.Relay,
			Discovery:                request.Discovery,
			OrganizationPrefix:       org.IpCidr,
			ReflexiveIPv4:            request.ReflexiveIPv4,
			EndpointLocalAddressIPv4: request.EndpointLocalAddressIPv4,
			SymmetricNat:             request.SymmetricNat,
			Hostname:                 request.Hostname,
		}

		if res := tx.Create(&device); res.Error != nil {
			return res.Error
		}
		span.SetAttributes(
			attribute.String("id", device.ID.String()),
		)
		return nil
	})

	if err != nil {
		var duplicate errDuplicateDevice
		if errors.Is(err, errUserOrOrgNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotAllowedError("user or organization"))
		} else if errors.As(err, &duplicate) {
			c.JSON(http.StatusConflict, models.NewConflictsError(duplicate.ID))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
	}

	c.JSON(http.StatusCreated, device)
}

// DeleteDevice handles deleting an existing device and associated ipam lease
// @Summary      Delete Device
// @Description  Deletes an existing device and associated IPAM lease
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      204  {object}  models.Device
// @Failure      400  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /devices/{id} [delete]
func (api *API) DeleteDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteDevice")
	defer span.End()
	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	device := models.Device{}
	if res := api.db.
		Scopes(api.DeviceIsVisibleToCurrentUser(c)).
		First(&device, "id = ?", deviceID); res.Error != nil {

		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("device"))
		} else {
			c.JSON(http.StatusBadRequest, models.NewApiInternalError(res.Error))
		}
		return
	}

	ipamAddress := device.TunnelIP
	orgID := device.OrganizationID
	orgPrefix := device.OrganizationPrefix
	childPrefix := device.ChildPrefix

	if res := api.db.WithContext(ctx).Delete(&device, "id = ?", device.Base.ID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.NewApiInternalError(res.Error))
		return
	}

	if ipamAddress != "" && orgPrefix != "" {
		if err := api.ipam.ReleaseToPool(c.Request.Context(), orgID, ipamAddress, orgPrefix); err != nil {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to release address to pool: %w", err)))
			return
		}
	}

	for _, prefix := range childPrefix {
		if err := api.ipam.ReleasePrefix(c.Request.Context(), orgID, prefix); err != nil {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to release child prefix: %w", err)))
			return
		}
	}

	c.JSON(http.StatusOK, device)
}
