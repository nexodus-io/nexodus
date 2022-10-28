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
	ID    string   `json:"id"`
	Peers []string `json:"peer-ids"`
}

func (ct *Controller) handleListDevices(c *gin.Context) {
	devices := make([]Device, 0)
	result := ct.db.Find(&devices)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
	}
	results := make([]DeviceJSON, 0)
	for _, d := range devices {
		var peers []string
		ct.db.Model(&Peer{}).Where("device_id = ?", d.ID).Pluck("id", &peers)
		results = append(results, DeviceJSON{
			ID:    d.ID,
			Peers: peers,
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
	var peers []string
	ct.db.Model(&Peer{}).Where("device_id = ?", device.ID).Pluck("id", &peers)
	results := DeviceJSON{
		ID:    device.ID,
		Peers: peers,
	}
	c.JSON(http.StatusOK, results)
}

type DeviceRequest struct {
	PublicKey string `json:"name"`
}

func (ct *Controller) handlePostDevices(c *gin.Context) {
	var request DeviceRequest
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if request.PublicKey == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the request did not contain a valid public key"})
		return
	}

	device := &Device{
		ID:        uuid.New().String(),
		PublicKey: request.PublicKey,
	}
	err := ct.db.Create(device)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	c.IndentedJSON(http.StatusCreated, device)
}
