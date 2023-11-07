package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/nexodus-io/nexodus/internal/wgcrypto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errOrgNotFound           = errors.New("organization not found")
	errUserNotFound          = errors.New("user not found")
	errDeviceNotFound        = errors.New("device not found")
	errInvitationNotFound    = errors.New("invitation not found")
	errSecurityGroupNotFound = errors.New("security group not found")
	errRegKeyExhausted       = errors.New("single use reg key exhausted")
)

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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/devices [get]
func (api *API) ListDevices(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListDevices")
	defer span.End()
	devices := make([]models.Device, 0)

	db := api.db.WithContext(ctx)
	db = api.DeviceIsOwnedByCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.Device{}, c, "hostname")
	result := db.Find(&devices)
	if result.Error != nil {
		api.SendInternalServerError(c, errors.New("error fetching keys from db"))
		return
	}

	tokenClaims, err := NxodusClaims(c, api.db.WithContext(ctx))
	if err != nil {
		c.JSON(err.Status, err.Body)
		return
	}

	// only show the device token when using the reg token that created the device.
	for i := range devices {
		hideDeviceBearerToken(&devices[i], tokenClaims)
	}
	c.JSON(http.StatusOK, devices)
}

func encryptDeviceBearerToken(token string, publicKey string) string {
	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return ""
	}
	sealed, err := wgcrypto.SealV1(key[:], []byte(token))
	if err != nil {
		return ""
	}

	return sealed.String()
}

func hideDeviceBearerToken(device *models.Device, claims *models.NexodusClaims) {
	if claims == nil {
		device.BearerToken = ""
		return
	}
	switch claims.Scope {
	case "reg-token":
		if claims.ID == device.RegKeyID.String() {
			device.BearerToken = encryptDeviceBearerToken(device.BearerToken, device.PublicKey)
			return
		}
	case "device-token":
		if claims.ID == device.ID.String() {
			device.BearerToken = encryptDeviceBearerToken(device.BearerToken, device.PublicKey)
			return
		}
	}
	device.BearerToken = ""
}

func (api *API) DeviceIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	return db.Where("owner_id = ?", userId)
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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

	db := api.db.WithContext(ctx)
	db = api.DeviceIsOwnedByCurrentUser(c, db)
	result := db.First(&device, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/devices/{id} [patch]
func (api *API) UpdateDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateDevice", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	deviceId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var request models.UpdateDevice

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	var device models.Device
	var tokenClaims *models.NexodusClaims
	err = api.transaction(ctx, func(tx *gorm.DB) error {

		db := api.DeviceIsOwnedByCurrentUser(c, tx)
		db = FilterAndPaginate(db, &models.Device{}, c, "hostname")

		result := db.First(&device, "id = ?", deviceId)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return errDeviceNotFound
		}

		var err2 *ApiResponseError
		tokenClaims, err2 = NxodusClaims(c, tx)
		if err2 != nil {
			return err2
		}

		if tokenClaims != nil {
			switch tokenClaims.Scope {
			case "reg-token":
				if tokenClaims.ID != device.RegKeyID.String() {
					return NewApiResponseError(http.StatusForbidden, models.NewApiError(errors.New("reg key does not have access")))
				}
			case "device-token":
				if tokenClaims.ID != device.ID.String() {
					return NewApiResponseError(http.StatusForbidden, models.NewApiError(errors.New("reg key does not have access")))
				}
			}
		}

		var vpc models.VPC
		if result = tx.First(&vpc, "id = ?", device.VpcID); result.Error != nil {
			return result.Error
		}

		originalIpamNamespace := defaultIPAMNamespace
		if vpc.PrivateCidr {
			originalIpamNamespace = vpc.ID
		}

		if request.Hostname != "" {
			device.Hostname = request.Hostname
		}

		if len(request.Endpoints) > 0 {
			device.Endpoints = request.Endpoints
		}

		// TODO: re-enable this when we are ready to support changing a device's VPC.

		if request.VpcID != nil && *request.VpcID != device.OrganizationID {

			var newVpc models.VPC
			if result := api.VPCIsReadableByCurrentUser(c, tx).
				Preload("Organization").
				First(&newVpc, "id = ?", request.VpcID); result.Error != nil {
				return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("newVpc"))
			}

			newIpamNamespace := defaultIPAMNamespace
			if newVpc.PrivateCidr {
				newIpamNamespace = newVpc.ID
			}

			// We can reuse the ip address if the ipam namespace is not changing.
			if originalIpamNamespace != newIpamNamespace {

				for _, t := range append(device.IPv4TunnelIPs, device.IPv6TunnelIPs...) {
					address := t.Address
					cidr := t.CIDR
					if address != "" && cidr != "" {
						if err := api.ipam.ReleaseToPool(c.Request.Context(), originalIpamNamespace, address, cidr); err != nil {
							return fmt.Errorf("failed to release the ip address to pool: %w", err)
						}
					}
				}
				for _, cidr := range device.AdvertiseCidrs {
					if err := api.ipam.ReleaseCIDR(c.Request.Context(), originalIpamNamespace, cidr); err != nil {
						return fmt.Errorf("failed to release cidr: %w", err)
					}
				}

				device.IPv4TunnelIPs[0].CIDR = newVpc.Ipv4Cidr
				device.IPv4TunnelIPs[0].Address, err = api.ipam.AssignFromPool(ctx, newIpamNamespace, newVpc.Ipv4Cidr)
				if err != nil {
					return fmt.Errorf("failed to request ipam address: %w", err)
				}

				device.IPv6TunnelIPs[0].CIDR = newVpc.Ipv6Cidr
				device.IPv6TunnelIPs[0].Address, err = api.ipam.AssignFromPool(ctx, newIpamNamespace, newVpc.Ipv6Cidr)
				if err != nil {
					return fmt.Errorf("failed to request ipam address: %w", err)
				}

				// allocate a CIDR if requested
				for _, cidr := range request.AdvertiseCidrs {
					if !util.IsValidPrefix(cidr) {
						return fmt.Errorf("invalid cidr detected in the advertise_cidrs field of %s", cidr)
					}
					// Skip the prefix assignment if it's an IPv4 or IPv6 default route
					if !util.IsDefaultIPv4Route(cidr) && !util.IsDefaultIPv6Route(cidr) {
						if err := api.ipam.AssignCIDR(ctx, newIpamNamespace, cidr); err != nil {
							return fmt.Errorf("failed to assign cidr: %w", err)
						}
					}
				}
			}

			device.AllowedIPs, err = getAllowedIPs(device.IPv4TunnelIPs[0].Address, device.IPv6TunnelIPs[0].Address, device.Relay)
			if err != nil {
				return err
			}

			device.VpcID = *request.VpcID
		}
		if request.SymmetricNat != nil {
			device.SymmetricNat = *request.SymmetricNat
		}

		if request.SecurityGroupId != nil {
			var sg models.SecurityGroup
			if result := api.SecurityGroupIsReadableByCurrentUser(c, tx).
				First(&sg, "id = ?", *request.SecurityGroupId); result.Error != nil {
				return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("security_group_id"))
			}
			device.SecurityGroupId = *request.SecurityGroupId
		}

		// check if the updated device advertised CIDRs match the existing device advertised CIDRs
		if request.AdvertiseCidrs != nil && !advertiseCidrEquals(device.AdvertiseCidrs, request.AdvertiseCidrs) {
			cidrAllocated := make(map[string]struct{})
			for _, cidr := range device.AdvertiseCidrs {
				if !util.IsValidPrefix(cidr) {
					return fmt.Errorf("invalid cidr detected in the advertise_cidrs field of %s", cidr)
				}
				cidrAllocated[cidr] = struct{}{}
			}
			for _, cidr := range request.AdvertiseCidrs {
				isDefaultRoute := util.IsDefaultIPRoute(cidr)
				// If the prefix is not a default route, process IPAM allocation/release
				if isDefaultRoute {
					continue
				}
				// lookup miss of prefix means we need to release it
				if _, ok := cidrAllocated[cidr]; ok {
					if err := api.ipam.ReleaseCIDR(ctx, originalIpamNamespace, cidr); err != nil {
						return err
					}
				} else {
					// otherwise we need to allocate it
					if err := api.ipam.AssignCIDR(ctx, originalIpamNamespace, cidr); err != nil {
						return err
					}
				}
			}
			device.AdvertiseCidrs = request.AdvertiseCidrs

		}

		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Save(&device); res.Error != nil {
			return res.Error
		}

		return nil
	})

	if err != nil {
		var apiResponseError *ApiResponseError
		if errors.Is(err, errDeviceNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("device"))
		} else if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	hideDeviceBearerToken(&device, tokenClaims)

	api.signalBus.Notify(fmt.Sprintf("/devices/vpc=%s", device.VpcID.String()))
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/devices [post]
func (api *API) CreateDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "AddDevice")
	defer span.End()
	var request models.AddDevice
	// Call ShouldBindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	if request.PublicKey == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("public_key"))
		return
	}
	if request.VpcID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("vpc_id"))
		return
	}

	userId := api.GetCurrentUserID(c)
	var tokenClaims *models.NexodusClaims
	var device models.Device
	err := api.transaction(ctx, func(tx *gorm.DB) error {

		var vpc models.VPC
		if result := api.VPCIsReadableByCurrentUser(c, tx).
			Preload("Organization").
			First(&vpc, "id = ?", request.VpcID); result.Error != nil {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("vpc"))
		}

		res := tx.Where("public_key = ?", request.PublicKey).First(&device)
		if res.Error == nil {
			return NewApiResponseError(http.StatusConflict, models.NewConflictsError(device.ID.String()))
		}
		if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return res.Error
		}

		var err2 *ApiResponseError
		tokenClaims, err2 = NxodusClaims(c, tx)
		if err2 != nil {
			return err2
		}
		if tokenClaims != nil && tokenClaims.Scope != "reg-token" {
			tokenClaims = nil
		}

		deviceId := uuid.Nil
		regKeyID := uuid.Nil
		var err error
		if tokenClaims != nil {
			regKeyID, err = uuid.Parse(tokenClaims.ID)
			if err != nil {
				return NewApiResponseError(http.StatusBadRequest, fmt.Errorf("invalid reg key id"))
			}

			// is the user token restricted to operating on a single device?
			if tokenClaims.DeviceID != uuid.Nil {
				err = tx.Where("id = ?", tokenClaims.DeviceID).First(&device).Error
				if err == nil {
					// If we get here the device exists but has a different public key, so assume
					// the reg toke has been previously used.
					return NewApiResponseError(http.StatusBadRequest, models.NewApiError(errRegKeyExhausted))
				}

				deviceId = tokenClaims.DeviceID
			}

			if tokenClaims.VpcID != request.VpcID {
				return NewApiResponseError(http.StatusBadRequest, models.NewFieldValidationError("vpc_id", "does not match the reg key vpc_id"))
			}
		}
		if deviceId == uuid.Nil {
			deviceId = uuid.New()
		}

		ipamNamespace := defaultIPAMNamespace
		if vpc.PrivateCidr {
			ipamNamespace = vpc.ID
		}

		var relay bool
		// determine if the node joining is a relay node
		if request.Relay {
			relay = true
		}

		var ipamIP string
		var ipamIPv6 string

		// If this was a static address request
		// TODO: handle a user requesting an IP not in the IPAM prefix
		if len(request.IPv4TunnelIPs) > 1 {
			return NewApiResponseError(http.StatusBadRequest, models.NewFieldValidationError("tunnel_ips_v4", "can only specify a single IPv4 address request"))
		} else if len(request.IPv4TunnelIPs) == 1 {
			ipamIP, err = api.ipam.AssignSpecificTunnelIP(ctx, ipamNamespace, vpc.Ipv4Cidr, request.IPv4TunnelIPs[0].Address)
			if err != nil {
				return fmt.Errorf("failed to request specific ipam address: %w", err)
			}
		} else {
			ipamIP, err = api.ipam.AssignFromPool(ctx, ipamNamespace, vpc.Ipv4Cidr)
			if err != nil {
				return fmt.Errorf("failed to request ipam address: %w", err)
			}
		}

		// Currently only support v4 requesting of specific addresses
		ipamIPv6, err = api.ipam.AssignFromPool(ctx, ipamNamespace, vpc.Ipv6Cidr)
		if err != nil {
			return fmt.Errorf("failed to request ipam v6 address: %w", err)
		}

		// allocate a CIDR if requested
		for _, cidr := range request.AdvertiseCidrs {
			if !util.IsValidPrefix(cidr) {
				return fmt.Errorf("invalid cidr detected in the advertise_cidrs field of %s", cidr)
			}
			// Skip the prefix assignment if it's an IPv4 or IPv6 default route
			if !util.IsDefaultIPv4Route(cidr) && !util.IsDefaultIPv6Route(cidr) {
				if err := api.ipam.AssignCIDR(ctx, ipamNamespace, cidr); err != nil {
					return fmt.Errorf("failed to assign cidr: %w", err)
				}
			}
		}

		allowedIPs, err := getAllowedIPs(ipamIP, ipamIPv6, relay)
		if err != nil {
			return err
		}

		// lets use a wg private key as the token, since it should be hard to guess.
		deviceToken, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}

		device = models.Device{
			Base: models.Base{
				ID: deviceId,
			},
			OwnerID:        userId,
			VpcID:          vpc.ID,
			OrganizationID: vpc.OrganizationID,
			PublicKey:      request.PublicKey,
			Endpoints:      request.Endpoints,
			AllowedIPs:     allowedIPs,
			IPv4TunnelIPs: []models.TunnelIP{
				{
					Address: ipamIP,
					CIDR:    vpc.Ipv4Cidr,
				},
			},
			IPv6TunnelIPs: []models.TunnelIP{
				{
					Address: ipamIPv6,
					CIDR:    vpc.Ipv6Cidr,
				},
			},
			AdvertiseCidrs:  request.AdvertiseCidrs,
			Relay:           request.Relay,
			SymmetricNat:    request.SymmetricNat,
			Hostname:        request.Hostname,
			Os:              request.Os,
			SecurityGroupId: vpc.Organization.SecurityGroupId,
			RegKeyID:        regKeyID,
			BearerToken:     "DT:" + deviceToken.String(),
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
		var apiResponseError *ApiResponseError
		if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	hideDeviceBearerToken(&device, tokenClaims)

	api.signalBus.Notify(fmt.Sprintf("/devices/vpc=%s", device.VpcID.String()))
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
	db := api.db.WithContext(ctx)
	if res := api.DeviceIsOwnedByCurrentUser(c, db).
		First(&device, "id = ?", deviceID); res.Error != nil {

		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("device"))
		} else {
			c.JSON(http.StatusBadRequest, models.NewApiError(res.Error))
		}
		return
	}

	var vpc models.VPC
	result := db.
		First(&vpc, "id = ?", device.VpcID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		api.SendInternalServerError(c, result.Error)
	}

	ipamNamespace := defaultIPAMNamespace
	if vpc.PrivateCidr {
		ipamNamespace = vpc.ID
	}

	ipamAddress := device.IPv4TunnelIPs[0].Address
	orgPrefix := device.IPv4TunnelIPs[0].CIDR
	advertiseCidrs := device.AdvertiseCidrs

	if res := api.db.WithContext(ctx).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
		Delete(&device, "id = ?", device.Base.ID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.NewApiError(res.Error))
		return
	}

	api.signalBus.Notify(fmt.Sprintf("/devices/vpc=%s", device.VpcID.String()))

	if ipamAddress != "" && orgPrefix != "" {
		if err := api.ipam.ReleaseToPool(c.Request.Context(), ipamNamespace, ipamAddress, orgPrefix); err != nil {
			api.SendInternalServerError(c, fmt.Errorf("failed to release the v4 address to pool: %w", err))
			return
		}
	}

	for _, cidr := range advertiseCidrs {
		if err := api.ipam.ReleaseCIDR(c.Request.Context(), ipamNamespace, cidr); err != nil {
			api.SendInternalServerError(c, fmt.Errorf("failed to release cidr: %w", err))
			return
		}
	}

	ipamAddressV6 := device.IPv6TunnelIPs[0].Address
	orgPrefixV6 := device.IPv6TunnelIPs[0].CIDR

	if ipamAddressV6 != "" && orgPrefixV6 != "" {
		if err := api.ipam.ReleaseToPool(c.Request.Context(), ipamNamespace, ipamAddressV6, orgPrefixV6); err != nil {
			api.SendInternalServerError(c, fmt.Errorf("failed to release the v6 address to pool: %w", err))
			return
		}
	}

	device.BearerToken = ""
	c.JSON(http.StatusOK, device)
}

func advertiseCidrEquals(existingPrefix, newPrefix []string) bool {
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
