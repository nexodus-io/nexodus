package apexcontroller

import (
	"errors"
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
	peers := make([]Peer, 0)
	result := ct.db.Find(&peers)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching peers from db"})
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(peers)))
	c.JSON(http.StatusOK, peers)
}

// GetPeer locates a peer
func (ct *Controller) handleGetPeers(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone id is not a valid UUID"})
		return
	}
	var peer Peer
	result := ct.db.First(&peer, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, peer)
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

	userRaw, ok := c.Get(UserRecord)
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "no user record in context"})
		return
	}

	user, ok := userRaw.(User)
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "user in context is not correct type"})
		return
	}

	var d Device
	res := ct.db.Where("public_key == ?", request.PublicKey).First(d)
	if res.Error == nil {
		// implicit ok for already exists condition
		c.Status(http.StatusOK)
		return
	}
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "database error"})
	}

	device := &Device{
		ID:        uuid.New().String(),
		PublicKey: request.PublicKey,
		UserID:    user.ID,
	}

	if res := ct.db.Create(device); res.Error != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": res.Error})
		return
	}

	user.Devices = append(user.Devices, device)
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
		userRaw, ok := c.Get(UserRecord)
		if !ok {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "no user record in context"})
			return
		}

		user, ok = userRaw.(User)
		if !ok {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "user in context is not correct type"})
		}
	} else {
		if res := ct.db.First(&user, "id = ?", userId); res.Error != nil {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": res.Error})
			return
		}
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
		userRaw, ok := c.Get(UserRecord)
		if !ok {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "no user record in context"})
			return
		}

		user, ok = userRaw.(User)
		if !ok {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "user in context is not correct type"})
		}
	} else {
		if res := ct.db.Preload("Devices").First(&user, "id = ?", userId); res.Error != nil {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": res.Error})
			return
		}
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
