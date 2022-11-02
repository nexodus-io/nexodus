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

type ZoneRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IpCidr      string `json:"cidr"`
	HubZone     bool   `json:"hub-zone"`
}

func (ct *Controller) handlePostZones(c *gin.Context) {
	var request ZoneRequest
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if request.IpCidr == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the zone request did not contain a required CIDR prefix"})
		return
	}
	if request.Name == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the zone request did not contain a required name"})
		return
	}

	// Create the zone
	newZone, err := ct.NewZone(uuid.New().String(), request.Name, request.Description, request.IpCidr, request.HubZone)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "unable to create zone"})
		return
	}
	log.Debugf("New zone request [ %s ] and ipam [ %s ] request", newZone.Name, newZone.IpCidr)
	c.IndentedJSON(http.StatusCreated, newZone)
}

type ZoneJSON struct {
	ID          string   `json:"id"`
	Peers       []string `json:"peer-ids"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IpCidr      string   `json:"cidr"`
	HubZone     bool     `json:"hub-zone"`
}

func (ct *Controller) handleListZones(c *gin.Context) {
	var zones []Zone
	result := ct.db.Find(&zones)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching zones from db"})
		return
	}
	results := make([]ZoneJSON, 0)
	for _, z := range zones {
		var peers []string
		ct.db.Model(&Peer{}).Where("zone_id = ?", z.ID).Pluck("id", &peers)
		results = append(results, ZoneJSON{
			ID:          z.ID,
			Peers:       peers,
			Name:        z.Name,
			Description: z.Description,
			IpCidr:      z.IpCidr,
			HubZone:     z.HubZone,
		})
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(results)))
	c.JSON(http.StatusOK, results)
}

func (ct *Controller) handleGetZones(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone id is not a valid UUID"})
		return
	}
	var zone Zone
	result := ct.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	var peers []string
	ct.db.Model(&Peer{}).Where("zone_id = ?", zone.ID).Pluck("id", &peers)
	results := ZoneJSON{
		ID:          zone.ID,
		Peers:       peers,
		Name:        zone.Name,
		Description: zone.Description,
		IpCidr:      zone.IpCidr,
		HubZone:     zone.HubZone,
	}
	c.JSON(http.StatusOK, results)
}

func (ct *Controller) handleListPeers(c *gin.Context) {
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone id is not a valid UUID"})
		return
	}
	var zone Zone
	result := ct.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	peers := make([]Peer, 0)
	result = ct.db.Where("zone_id = ?", zone.ID).Find(&peers)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching peers from db"})
		return
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(peers)))
	c.JSON(http.StatusOK, peers)
}

func (ct *Controller) handleGetPeers(c *gin.Context) {
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone id is not a valid UUID"})
		return
	}
	var zone Zone
	result := ct.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone id is not a valid UUID"})
		return
	}
	var peer Peer
	result = ct.db.First(&peer, "id = ?", id.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, peer)
}

type PeerRequest struct {
	DeviceID    string `json:"device-id"`
	EndpointIP  string `json:"endpoint-ip"`
	AllowedIPs  string `json:"allowed-ips"`
	NodeAddress string `json:"node-address"`
	ChildPrefix string `json:"child-prefix"`
	HubRouter   bool   `json:"hub-router"`
	HubZone     bool   `json:"hub-zone"`
	ZonePrefix  string `json:"zone-prefix"`
}

func (ct *Controller) handlePostPeers(c *gin.Context) {
	ctx := c.Request.Context()
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone id is not a valid UUID"})
		return
	}

	var zone Zone
	if res := ct.db.Preload("Peers").First(&zone, "id = ?", k.String()); res.Error != nil {
		c.Status(http.StatusNotFound)
		return
	}

	var request PeerRequest

	if err := c.BindJSON(&request); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	tx := ct.db.Begin()

	var device Device
	if res := tx.First(&device, "id = ?", request.DeviceID); res.Error != nil {
		tx.Rollback()
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	var user User
	if res := tx.First(&user, "id = ?", device.UserID); res.Error != nil {
		tx.Rollback()
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	if user.ZoneID != zone.ID {
		c.IndentedJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("user id %s is not part of zone %s", user.ID, zone.ID)})
		return
	}

	ipamPrefix := zone.IpCidr
	var hubZone bool
	var hubRouter bool
	// determine if the node joining is a hub-router or in a hub-zone
	if request.HubRouter && zone.HubZone {
		hubRouter = true
		// If the zone already has a hub-router, reject the join of a new node trying to be a zone-router
		var hubRouterAlreadyExists string
		for _, p := range zone.Peers {
			// If the node joining is a re-join from the zone-router allow it
			if p.HubRouter && p.DeviceID != device.ID {
				hubRouterAlreadyExists = p.EndpointIP
				tx.Rollback()
				c.IndentedJSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("hub router already exists on the endpoint %s", hubRouterAlreadyExists)})
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
					c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "ipam error"})
					return
				}
			} else {
				if peer.NodeAddress == "" {
					c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "peer does not have a node address assigned in the peer table and did not request a specifc address"})
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
				c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "ipam error"})
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
				c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "ipam error"})
				return
			}
		} else {
			var err error
			ip, err = ct.assignFromPool(ctx, ipamPrefix)
			if err != nil {
				tx.Rollback()
				log.Error(err)
				c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "ipam error"})
				return
			}
		}
		// allocate a child prefix if requested
		if request.ChildPrefix != "" {
			if err := ct.assignChildPrefix(ctx, request.ChildPrefix); err != nil {
				tx.Rollback()
				log.Error(err)
				c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "ipam error"})
				return
			}
		}
		peer = &Peer{
			ID:          uuid.New().String(),
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
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	c.IndentedJSON(http.StatusCreated, peer)
}

type DeviceJSON struct {
	ID        string `json:"id"`
	PublicKey string `json:"public-key"`
	UserID    string `json:"user-id"`
}

func (ct *Controller) handleListDevices(c *gin.Context) {
	devices := make([]Device, 0)
	result := ct.db.Find(&devices)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	results := make([]DeviceJSON, 0)
	for _, d := range devices {
		results = append(results, DeviceJSON{
			ID:        d.ID,
			PublicKey: d.PublicKey,
			UserID:    d.UserID,
		})
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(results)))
	c.JSON(http.StatusOK, results)
}

func (ct *Controller) handleGetDevices(c *gin.Context) {
	k := c.Param("id")
	if k == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pubkey id is not valid"})
		return
	}
	var device Device
	result := ct.db.Debug().First(&device, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	results := DeviceJSON{
		ID:        device.ID,
		PublicKey: device.PublicKey,
		UserID:    device.UserID,
	}
	c.JSON(http.StatusOK, results)
}

type DeviceRequest struct {
	PublicKey string `json:"public-key"`
}

func (ct *Controller) handlePostDevices(c *gin.Context) {
	var request DeviceRequest
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}
	if request.PublicKey == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the request did not contain a valid public key"})
		return
	}

	userId := c.GetString(AuthUserID)
	var user User
	if res := ct.db.Preload("Devices").First(&user, "id = ?", userId); res.Error != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}

	var device Device
	res := ct.db.Where("public_key = ?", request.PublicKey).First(&device)
	if res.Error == nil {
		r := DeviceJSON{
			ID:        device.ID,
			PublicKey: device.PublicKey,
			UserID:    device.UserID,
		}
		c.IndentedJSON(http.StatusConflict, r)
		return
	}
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	device = Device{
		ID:        uuid.New().String(),
		PublicKey: request.PublicKey,
		UserID:    user.ID,
	}

	if res := ct.db.Create(&device); res.Error != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": res.Error})
		return
	}

	user.Devices = append(user.Devices, &device)
	ct.db.Save(&user)

	result := DeviceJSON{
		ID:        device.ID,
		UserID:    device.UserID,
		PublicKey: device.PublicKey,
	}

	c.IndentedJSON(http.StatusCreated, result)
}

type UserRequest struct {
	ZoneID string `json:"zone-id"`
}

func (ct *Controller) handlePatchUser(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is not valid"})
		return
	}
	var request UserRequest
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	if request.ZoneID == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the request did not contain valid data"})
		return
	}

	var user User
	if userId == "me" {
		userId = c.GetString(AuthUserID)
	}

	if res := ct.db.First(&user, "id = ?", userId); res.Error != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": res.Error})
		return
	}

	var zone Zone
	if res := ct.db.First(&zone, "id = ?", request.ZoneID); res.Error != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "zone id is not valid"})
		return
	}

	user.ZoneID = request.ZoneID

	ct.db.Save(&user)

	c.Status(http.StatusOK)
}

type UserJSON struct {
	ID      string   `json:"id"`
	Devices []string `json:"devices"`
	ZoneID  string   `json:"zone-id"`
}

func (ct *Controller) handleGetUser(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is not valid"})
		return
	}

	var user User
	if userId == "me" {
		userId = c.GetString(AuthUserID)
	}

	if res := ct.db.Preload("Devices").First(&user, "id = ?", userId); res.Error != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": res.Error})
		return
	}

	var devices []string
	for _, d := range user.Devices {
		devices = append(devices, d.ID)
	}
	result := UserJSON{
		ID:      user.ID,
		Devices: devices,
		ZoneID:  user.ZoneID,
	}

	c.JSON(http.StatusOK, result)
}
