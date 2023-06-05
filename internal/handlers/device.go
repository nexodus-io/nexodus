package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errUserOrOrgNotFound     = errors.New("user or organization not found")
	errOrgNotFound           = errors.New("organization not found")
	errUserNotFound          = errors.New("user not found")
	errDeviceNotFound        = errors.New("device not found")
	errInvitationNotFound    = errors.New("invitation not found")
	errSecurityGroupNotFound = errors.New("security group not found")
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
// @Id  		 ListDevices
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.Device
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/devices [get]
func (api *API) ListDevices(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListDevices")
	defer span.End()
	devices := make([]models.Device, 0)

	result := api.db.WithContext(ctx).Scopes(
		api.DeviceIsOwnedByCurrentUser(c),
		FilterAndPaginate(&models.Device{}, c, "hostname"),
	).Find(&devices)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	c.JSON(http.StatusOK, devices)
}

func (api *API) DeviceIsOwnedByCurrentUser(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)

		// this could potentially be driven by rego output

		return db.Where("user_id = ?", userId)
		//if api.dialect == database.DialectSqlLite {
		//	return db.Where("user_id = ? OR organization_id in (SELECT organization_id FROM user_organizations where user_id=?)", userId, userId)
		//} else {
		//	return db.Where("user_id = ? OR organization_id::text in (SELECT organization_id::text FROM user_organizations where user_id=?)", userId, userId)
		//}
	}
}

// GetDevice gets a device by ID
// @Summary      Get Devices
// @Description  Gets a device by ID
// @Id  		 GetDevice
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      200  {object}  models.Device
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/devices/{id} [get]
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
		Scopes(api.DeviceIsOwnedByCurrentUser(c)).
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
// @Id  		 UpdateDevice
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Param		 update body models.UpdateDevice true "Device Update"
// @Success      200  {object}  models.Device
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/devices/{id} [patch]
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
			Scopes(api.DeviceIsOwnedByCurrentUser(c)).
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

		if len(request.Endpoints) > 0 {
			device.Endpoints = request.Endpoints
		}

		if request.OrganizationID != uuid.Nil && request.OrganizationID != device.OrganizationID {
			userId := c.GetString(gin.AuthUserKey)

			var org models.Organization
			if res := tx.Model(&org).
				Joins("inner join user_organizations on user_organizations.organization_id=organizations.id").
				Where("user_organizations.user_id=? AND organizations.id=?", userId, request.OrganizationID).
				First(&org); res.Error != nil {
				return errUserOrOrgNotFound
			}

			if err := api.ipam.ReleaseToPool(c.Request.Context(), device.OrganizationID, device.TunnelIP, device.OrganizationPrefix); err != nil {
				c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to release the v4 address to pool: %w", err)))
				return err
			}

			if err := api.ipam.ReleaseToPool(c.Request.Context(), device.OrganizationID, device.TunnelIpV6, device.OrganizationPrefixV6); err != nil {
				c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to release the v6 address to pool: %w", err)))
				return err
			}

			device.TunnelIP, err = api.ipam.AssignFromPool(ctx, org.ID, org.IpCidr)
			if err != nil {
				return fmt.Errorf("failed to request ipam address: %w", err)
			}
			device.OrganizationPrefix = org.IpCidr

			device.TunnelIpV6, err = api.ipam.AssignFromPool(ctx, org.ID, org.IpCidrV6)
			if err != nil {
				return fmt.Errorf("failed to request ipam v6 address: %w", err)
			}
			device.OrganizationPrefixV6 = org.IpCidrV6

			device.AllowedIPs, err = getAllowedIPs(device.TunnelIP, device.TunnelIpV6, device.Relay)
			if err != nil {
				return err
			}

			device.OrganizationID = request.OrganizationID
		}

		device.SymmetricNat = request.SymmetricNat

		// check if the updated device child prefix matches the existing device prefix
		if request.ChildPrefix != nil && !childPrefixEquals(device.ChildPrefix, request.ChildPrefix) {
			prefixAllocated := make(map[string]struct{})
			for _, prefix := range device.ChildPrefix {
				if !util.IsValidPrefix(prefix) {
					return fmt.Errorf("invalid cidr detected in the child prefix field of %s", prefix)
				}
				prefixAllocated[prefix] = struct{}{}
			}
			for _, prefix := range request.ChildPrefix {
				isDefaultRoute := util.IsDefaultIPRoute(prefix)
				// If the prefix is not a default route, process IPAM allocation/release
				if isDefaultRoute {
					continue
				}
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

		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Save(&device); res.Error != nil {
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

	api.signalBus.Notify(fmt.Sprintf("/devices/org=%s", device.OrganizationID.String()))
	c.JSON(http.StatusOK, device)
}

func getAllowedIPs(ip string, ip6 string, relay bool) ([]string, error) {
	var err error

	hostPrefixV4 := ip
	hostPrefixV6 := ip6

	// append a host prefix length to the leased v4 IPAM address to add to the allowed-ips slice
	if !relay {
		hostPrefixV4, err = util.AppendPrefixMask(hostPrefixV4, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to append a v4 prefix length to the IPAM address: %w", err)
		}
	}
	// append a host prefix length to the leased v6 IPAM address to add to the allowed-ips slice
	if !relay {
		hostPrefixV6, err = util.AppendPrefixMask(hostPrefixV6, 128)
		if err != nil {
			return nil, fmt.Errorf("failed to append a v4 prefix length to the IPAM address: %w", err)
		}
	}

	var allowedIPs []string
	// append the IPAM leases to the allowed-ips list to be distributed to peers
	allowedIPs = append(allowedIPs, hostPrefixV4)
	allowedIPs = append(allowedIPs, hostPrefixV6)

	return allowedIPs, nil
}

// CreateDevice handles adding a new device
// @Summary      Add Devices
// @Id  		 CreateDevice
// @Tags         Devices
// @Description  Adds a new device
// @Accept       json
// @Produce      json
// @Param        Device  body   models.AddDevice  true "Add Device"
// @Success      201  {object}  models.Device
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /api/devices [post]
func (api *API) CreateDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "AddDevice")
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

		var relay bool
		// determine if the node joining is a relay node
		if request.Relay {
			relay = true
		}

		var ipamIP string
		var ipamIPv6 string
		var err error
		// If this was a static address request
		// TODO: handle a user requesting an IP not in the IPAM prefix
		if request.TunnelIP != "" {
			ipamIP, err = api.ipam.AssignSpecificTunnelIP(ctx, org.ID, org.IpCidr, request.TunnelIP)
			if err != nil {
				return fmt.Errorf("failed to request specific ipam address: %w", err)
			}
		} else {
			ipamIP, err = api.ipam.AssignFromPool(ctx, org.ID, org.IpCidr)
			if err != nil {
				return fmt.Errorf("failed to request ipam address: %w", err)
			}
		}
		// Currently only support v4 requesting of specific addresses
		ipamIPv6, err = api.ipam.AssignFromPool(ctx, org.ID, org.IpCidrV6)
		if err != nil {
			return fmt.Errorf("failed to request ipam v6 address: %w", err)
		}

		// allocate a child prefix if requested
		for _, prefix := range request.ChildPrefix {
			if !util.IsValidPrefix(prefix) {
				return fmt.Errorf("invalid cidr detected in the child prefix field of %s", prefix)
			}
			// Skip the prefix assignment if it's an IPv4 or IPv6 default route
			if !util.IsDefaultIPv4Route(prefix) && !util.IsDefaultIPv6Route(prefix) {
				if err := api.ipam.AssignPrefix(ctx, org.ID, prefix); err != nil {
					return fmt.Errorf("failed to assign child prefix: %w", err)
				}
			}
		}

		allowedIPs, err := getAllowedIPs(ipamIP, ipamIPv6, relay)
		if err != nil {
			return err
		}

		device = models.Device{
			UserID:                   userId,
			OrganizationID:           org.ID,
			PublicKey:                request.PublicKey,
			Endpoints:                request.Endpoints,
			AllowedIPs:               allowedIPs,
			TunnelIP:                 ipamIP,
			TunnelIpV6:               ipamIPv6,
			ChildPrefix:              request.ChildPrefix,
			Relay:                    request.Relay,
			Discovery:                request.Discovery,
			OrganizationPrefix:       org.IpCidr,
			OrganizationPrefixV6:     org.IpCidrV6,
			EndpointLocalAddressIPv4: request.EndpointLocalAddressIPv4,
			SymmetricNat:             request.SymmetricNat,
			Hostname:                 request.Hostname,
			Os:                       request.Os,
			SecurityGroupId:          org.SecurityGroupId,
		}

		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Create(&device); res.Error != nil {
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

	api.signalBus.Notify(fmt.Sprintf("/devices/org=%s", device.OrganizationID.String()))
	c.JSON(http.StatusCreated, device)
}

// DeleteDevice handles deleting an existing device and associated ipam lease
// @Summary      Delete Device
// @Description  Deletes an existing device and associated IPAM lease
// @Id 			 DeleteDevice
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      204  {object}  models.Device
// @Failure      400  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /api/devices/{id} [delete]
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
		Scopes(api.DeviceIsOwnedByCurrentUser(c)).
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

	if res := api.db.WithContext(ctx).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
		Delete(&device, "id = ?", device.Base.ID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.NewApiInternalError(res.Error))
		return
	}

	api.signalBus.Notify(fmt.Sprintf("/devices/org=%s", device.OrganizationID.String()))

	if ipamAddress != "" && orgPrefix != "" {
		if err := api.ipam.ReleaseToPool(c.Request.Context(), orgID, ipamAddress, orgPrefix); err != nil {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to release the v4 address to pool: %w", err)))
			return
		}
	}

	for _, prefix := range childPrefix {
		if err := api.ipam.ReleasePrefix(c.Request.Context(), orgID, prefix); err != nil {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to release child prefix: %w", err)))
			return
		}
	}

	ipamAddressV6 := device.TunnelIpV6
	orgPrefixV6 := device.OrganizationPrefixV6

	if ipamAddressV6 != "" && orgPrefixV6 != "" {
		if err := api.ipam.ReleaseToPool(c.Request.Context(), orgID, ipamAddressV6, orgPrefixV6); err != nil {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to release the v6 address to pool: %w", err)))
			return
		}
	}

	c.JSON(http.StatusOK, device)
}

func childPrefixEquals(existingPrefix, newPrefix []string) bool {
	if len(existingPrefix) != len(newPrefix) {
		return false
	}
	countMap := make(map[string]int)
	for _, value := range existingPrefix {
		countMap[value]++
	}
	for _, value := range newPrefix {
		countMap[value]--
		if countMap[value] < 0 {
			return false
		}
	}
	return true
}
