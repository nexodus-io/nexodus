package apexcontroller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// handlePostZones creates a new Zone
// @Summary      Create a Zone
// @Description  Creates a named zone with the given CIDR
// @Tags         Zone
// @Accept       json
// @Produce      json
// @Param        zone  body     AddZone  true  "Add Zone"
// @Success      201  {object}  Zone
// @Failure      400  {object}  ApiError
// @Failure		 401  {object}  ApiError
// @Failure      500  {object}  ApiError
// @Router       /zones [post]
func (ct *Controller) handlePostZones(c *gin.Context) {
	var request AddZone
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: err.Error()})
		return
	}
	if request.IpCidr == "" {
		c.JSON(http.StatusBadRequest, ApiError{Error: "the zone request did not contain a required CIDR prefix"})
		return
	}
	if request.Name == "" {
		c.JSON(http.StatusBadRequest, ApiError{Error: "the zone request did not contain a required name"})
		return
	}

	// Create the zone
	newZone, err := ct.NewZone(uuid.New().String(), request.Name, request.Description, request.IpCidr, request.HubZone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Error: "unable to create zone"})
		return
	}
	log.Debugf("New zone request [ %s ] and ipam [ %s ] request", newZone.Name, newZone.IpCidr)
	c.JSON(http.StatusCreated, newZone)
}

// handleListZones lists all zones
// @Summary      List Zones
// @Description  Lists all zones
// @Tags         Zone
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []Zone
// @Failure		 401  {object}  ApiError
// @Failure		 500  {object}  ApiError
// @Router       /zones [get]
func (ct *Controller) handleListZones(c *gin.Context) {
	var zones []Zone
	result := ct.db.Find(&zones)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Error: "error fetching zones from db"})
		return
	}
	for _, z := range zones {
		var peers []uuid.UUID
		ct.db.Model(&Peer{}).Where("zone_id = ?", z.ID).Pluck("id", &peers)
		z.PeerList = peers
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(zones)))
	c.JSON(http.StatusOK, zones)
}

// handleGetZones gets a specific zone
// @Summary      Get Zones
// @Description  Gets a Zone by Zone ID
// @Tags         Zone
// @Accepts		 json
// @Produce      json
// @Param		 id   path      string true "Zone ID"
// @Success      200  {object}  Zone
// @Failure      400  {object}  ApiError
// @Failure		 401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Router       /zones/{id} [get]
func (ct *Controller) handleGetZones(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: "zone id is not a valid UUID"})
		return
	}
	var zone Zone
	result := ct.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, ApiError{Error: "zone not found"})
		return
	}
	var peers []uuid.UUID
	ct.db.Model(&Peer{}).Where("zone_id = ?", zone.ID).Pluck("id", &peers)
	zone.PeerList = peers
	c.JSON(http.StatusOK, zone)
}

// handleListPeers lists all peers in a zone
// @Summary      List Peers
// @Description  Lists all peers for this zone
// @Tags         Peers
// @Accepts		 json
// @Produce      json
// @Param		 id   path       string true "Zone ID"
// @Success      200  {object}  []Peer
// @Failure      400  {object}  ApiError
// @Failure		 401  {object}  ApiError
// @Failure		 500  {object}  ApiError
// @Router       /zones/{id}/peers [get]
func (ct *Controller) handleListPeers(c *gin.Context) {
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: "zone id is not a valid UUID"})
		return
	}
	var zone Zone
	result := ct.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, ApiError{Error: "zone not found"})
		return
	}
	peers := make([]Peer, 0)
	result = ct.db.Where("zone_id = ?", zone.ID).Find(&peers)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Error: "error fetching peers from db"})
		return
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(peers)))
	c.JSON(http.StatusOK, peers)
}

// handleGetPeers gets a peer in a zone
// @Summary      Get Peer
// @Description  Gets a peer in a zone by ID
// @Tags         Peers
// @Accepts		 json
// @Produce      json
// @Param		 zone_id path   string true "Zone ID"
// @Param		 peer_id path   string true "Zone ID"
// @Success      200  {object}  []Peer
// @Failure      400  {object}  ApiError
// @Failure		 401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Failure		 500  {object}  ApiError
// @Router       /zones/{zone_id}/peers/{peer_id} [get]
func (ct *Controller) handleGetPeers(c *gin.Context) {
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: "zone id is not a valid UUID"})
		return
	}
	var zone Zone
	result := ct.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, ApiError{Error: "zone not found"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: "zone id is not a valid UUID"})
		return
	}
	var peer Peer
	result = ct.db.First(&peer, "id = ?", id.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, ApiError{Error: "peer not found"})
		return
	}
	c.JSON(http.StatusOK, peer)
}

// handlePostPeers adds a new peer in a zone
// @Summary      Add Peer
// @Description  Adds a new Peer in this Zone
// @Tags         Peers
// @Accepts		 json
// @Produce      json
// @Param		 zone_id path   string true "Zone ID"
// @Param		 zone    body   AddZone true "Add Zone"
// @Success      201  {object}  Peer
// @Failure      400  {object}  ApiError
// @Failure		 401  {object}  ApiError
// @Failure		 403  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Failure      409  {object}  Peer
// @Failure		 500  {object}  ApiError
// @Router       /zones/{zone_id}/peers [post]
func (ct *Controller) handlePostPeers(c *gin.Context) {
	ctx := c.Request.Context()
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: "zone id is not a valid UUID"})
		return
	}

	var zone Zone
	if res := ct.db.Preload("Peers").First(&zone, "id = ?", k.String()); res.Error != nil {
		c.JSON(http.StatusNotFound, ApiError{Error: "zone not found"})
		return
	}

	var request AddPeer
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: err.Error()})
		return
	}

	tx := ct.db.Begin()

	var device Device
	if res := tx.First(&device, "id = ?", request.DeviceID); res.Error != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, ApiError{Error: "database error"})
		return
	}

	var user User
	if res := tx.First(&user, "id = ?", device.UserID); res.Error != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, ApiError{Error: "database error"})
		return
	}

	if user.ZoneID != zone.ID {
		c.JSON(http.StatusForbidden, ApiError{Error: fmt.Sprintf("user id %s is not part of zone %s", user.ID, zone.ID)})
		return
	}

	ipamPrefix := zone.IpCidr
	var hubZone bool
	var hubRouter bool
	// determine if the node joining is a hub-router or in a hub-zone
	if request.HubRouter && zone.HubZone {
		hubRouter = true
		// If the zone already has a hub-router, reject the join of a new node trying to be a zone-router
		for _, p := range zone.Peers {
			// If the node joining is a re-join from the zone-router allow it
			if p.HubRouter && p.DeviceID != device.ID {
				tx.Rollback()
				c.JSON(http.StatusConflict, p)
				return
			}
		}
	}

	if zone.HubZone {
		hubZone = true
	}

	var found bool
	var peer *Peer
	for _, p := range zone.Peers {
		if p.DeviceID == device.ID {
			found = true
			peer = p
			break
		}
	}
	if found {
		peer.EndpointIP = request.EndpointIP

		if request.NodeAddress != peer.NodeAddress {
			var ip string
			if request.NodeAddress != "" {
				var err error
				ip, err = ct.assignSpecificNodeAddress(ctx, ipamPrefix, request.NodeAddress)
				if err != nil {
					tx.Rollback()
					log.Error(err)
					c.JSON(http.StatusInternalServerError, ApiError{Error: "ipam error"})
					return
				}
			} else {
				if peer.NodeAddress == "" {
					c.JSON(http.StatusBadRequest, ApiError{Error: "peer does not have a node address assigned in the peer table and did not request a specifc address"})
					return
				}
				ip = peer.NodeAddress
			}
			peer.NodeAddress = ip
			peer.AllowedIPs = ip
		}

		if request.ChildPrefix != peer.ChildPrefix {
			if err := ct.assignChildPrefix(ctx, request.ChildPrefix); err != nil {
				tx.Rollback()
				log.Error(err)
				c.JSON(http.StatusInternalServerError, ApiError{Error: "ipam error"})
				return
			}
		}
	} else {
		log.Debugf("Public key not in the zone %s. Creating a new peer", zone.ID)
		var ip string
		// If this was a static address request
		// TODO: handle a user requesting an IP not in the IPAM prefix
		if request.NodeAddress != "" {
			var err error
			ip, err = ct.assignSpecificNodeAddress(ctx, ipamPrefix, request.NodeAddress)
			if err != nil {
				tx.Rollback()
				log.Error(err)
				c.JSON(http.StatusInternalServerError, ApiError{Error: "ipam error"})
				return
			}
		} else {
			var err error
			ip, err = ct.assignFromPool(ctx, ipamPrefix)
			if err != nil {
				tx.Rollback()
				log.Error(err)
				c.JSON(http.StatusInternalServerError, ApiError{Error: "ipam error"})
				return
			}
		}
		// allocate a child prefix if requested
		if request.ChildPrefix != "" {
			if err := ct.assignChildPrefix(ctx, request.ChildPrefix); err != nil {
				tx.Rollback()
				log.Error(err)
				c.JSON(http.StatusInternalServerError, ApiError{Error: "ipam error"})
				return
			}
		}
		peer = &Peer{
			DeviceID:    device.ID,
			ZoneID:      zone.ID,
			EndpointIP:  request.EndpointIP,
			AllowedIPs:  ip,
			NodeAddress: ip,
			ChildPrefix: request.ChildPrefix,
			ZonePrefix:  ipamPrefix,
			HubZone:     hubZone,
			HubRouter:   hubRouter,
		}
		tx.Create(peer)
		zone.Peers = append(zone.Peers, peer)
		tx.Save(&zone)
	}
	tx.Save(&peer)

	if err := tx.Commit(); err.Error != nil {
		tx.Rollback()
		log.Error(err.Error)
		c.JSON(http.StatusInternalServerError, ApiError{Error: "database error"})
		return
	}

	c.JSON(http.StatusCreated, peer)
}

// handleListDevices lists all devices
// @Summary      List Devices
// @Description  Lists all devices
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []Device
// @Failure		 401  {object}  ApiError
// @Router       /devices [get]
func (ct *Controller) handleListDevices(c *gin.Context) {
	devices := make([]Device, 0)
	result := ct.db.Find(&devices)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(devices)))
	c.JSON(http.StatusOK, devices)
}

// handleGetDevices lists all devices
// @Summary      Get Devices
// @Description  Gets a device by its ID
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      200  {object}  Device
// @Failure		 401  {object}  ApiError
// @Failure      400  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Router       /devices/{id} [get]
func (ct *Controller) handleGetDevices(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: "id is not valid"})
		return
	}
	var device Device
	result := ct.db.Debug().First(&device, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, device)
}

// handlePostDevices handles adding a new device
// @Summary      Add Devices
// @Description  Adds a new device
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        device  body   AddDevice  true "Add Device"
// @Success      201  {object}  Device
// @Failure      400  {object}  ApiError
// @Failure		 401  {object}  ApiError
// @Failure      409  {object}  Device
// @Failure      500  {object}  ApiError
// @Router       /devices [post]
func (ct *Controller) handlePostDevices(c *gin.Context) {
	var request AddDevice
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: err.Error()})
		return
	}
	if request.PublicKey == "" {
		c.JSON(http.StatusBadRequest, ApiError{Error: "the request did not contain a valid public key"})
		return
	}

	userId := c.GetString(AuthUserID)
	var user User
	if res := ct.db.Preload("Devices").First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Error: "user not found"})
		return
	}

	var device Device
	res := ct.db.Where("public_key = ?", request.PublicKey).First(&device)
	if res.Error == nil {
		c.JSON(http.StatusConflict, device)
		return
	}
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, ApiError{Error: "database error"})
		return
	}

	device = Device{
		PublicKey: request.PublicKey,
		UserID:    user.ID,
	}

	if res := ct.db.Create(&device); res.Error != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Error: res.Error.Error()})
		return
	}

	user.Devices = append(user.Devices, &device)
	ct.db.Save(&user)

	c.JSON(http.StatusCreated, device)
}

// handlePatchUser handles moving a User to a new Zone
// @Summary      Update User
// @Description  Changes the users zone
// @Tags         User
// @Accepts		 json
// @Produce      json
// @Param        id  path       string  true  "User ID"
// @Param        device  body   PatchUser  true "Patch User"
// @Success      200  {object}  User
// @Failure      400  {object}  ApiError
// @Failure		 401  {object}  ApiError
// @Failure      500  {object}  ApiError
// @Router       /users/{id} [patch]
func (ct *Controller) handlePatchUser(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		c.JSON(http.StatusBadRequest, ApiError{Error: "user id is not valid"})
		return
	}
	var request PatchUser
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: err.Error()})
		return
	}

	var user User
	if userId == "me" {
		userId = c.GetString(AuthUserID)
	}

	if res := ct.db.First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Error: res.Error.Error()})
		return
	}

	var zone Zone
	if res := ct.db.First(&zone, "id = ?", request.ZoneID); res.Error != nil {
		c.JSON(http.StatusBadRequest, ApiError{Error: "zone id is not valid"})
		return
	}

	user.ZoneID = request.ZoneID

	ct.db.Save(&user)

	c.JSON(http.StatusOK, user)
}

// handleGetUser gets a user
// @Summary      Get User
// @Description  Gets a user
// @Tags         User
// @Accepts		 json
// @Produce      json
// @Param        id  path       string  true  "User ID"
// @Success      200  {object}  User
// @Failure      400  {object}  ApiError
// @Failure		 401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Failure      500  {object}  ApiError
// @Router       /users/{id} [get]
func (ct *Controller) handleGetUser(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		c.JSON(http.StatusBadRequest, ApiError{Error: "user id is not valid"})
		return
	}

	var user User
	if userId == "me" {
		userId = c.GetString(AuthUserID)
	}

	if res := ct.db.Preload("Devices").First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusNotFound, ApiError{Error: res.Error.Error()})
		return
	}

	var devices []uuid.UUID
	for _, d := range user.Devices {
		devices = append(devices, d.ID)
	}
	user.DeviceList = devices

	c.JSON(http.StatusOK, user)
}
