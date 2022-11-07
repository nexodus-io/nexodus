package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	defaultZoneName        = "default"
	defaultZoneDescription = "Default Zone"
	defaultZonePrefix      = "10.200.1.0/20"
)

// CreateZone creates a new Zone
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
func (api *API) CreateZone(c *gin.Context) {
	ctx := c.Request.Context()
	var request models.AddZone
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: err.Error()})
		return
	}
	if request.IpCidr == "" {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "the zone request did not contain a required CIDR prefix"})
		return
	}
	if request.Name == "" {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "the zone request did not contain a required name"})
		return
	}

	// Create the zone
	if err := api.ipam.AssignPrefix(ctx, request.IpCidr); err != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: err.Error()})
		return
	}
	newZone := models.Zone{
		Peers:       make([]*models.Peer, 0),
		Name:        request.Name,
		Description: request.Description,
		IpCidr:      request.IpCidr,
		HubZone:     request.HubZone,
	}
	res := api.db.Create(&newZone)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "unable to create zone"})
		return
	}
	log.Debugf("New zone request [ %s ] and ipam [ %s ] request", newZone.Name, newZone.IpCidr)
	c.JSON(http.StatusCreated, newZone)
}

func (api *API) CreateDefaultZoneIfNotExists(ctx context.Context) (uuid.UUID, error) {
	var zone models.Zone
	res := api.db.Where("name = ?", defaultZoneName).First(&zone)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		log.Debug("Creating Default Zone")
		if err := api.ipam.AssignPrefix(ctx, defaultZonePrefix); err != nil {
			return uuid.Nil, err
		}
		zone.Name = defaultZoneName
		zone.Description = defaultZoneDescription
		zone.IpCidr = defaultZonePrefix
		api.db.Save(&zone)
	}
	log.Debugf("Default Zone UUID is: %s", zone.ID)
	return zone.ID, nil
}

// ListZones lists all zones
// @Summary      List Zones
// @Description  Lists all zones
// @Tags         Zone
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []Zone
// @Failure		 401  {object}  ApiError
// @Failure		 500  {object}  ApiError
// @Router       /zones [get]
func (api *API) ListZones(c *gin.Context) {
	var zones []models.Zone
	result := api.db.Scopes(FilterAndPaginate(&models.Zone{}, c)).Find(&zones)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "error fetching zones from db"})
		return
	}
	for _, z := range zones {
		var peers []uuid.UUID
		api.db.Model(&models.Peer{}).Where("zone_id = ?", z.ID).Pluck("id", &peers)
		z.PeerList = peers
	}
	c.JSON(http.StatusOK, zones)
}

// GetZones gets a specific zone
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
func (api *API) GetZones(c *gin.Context) {
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "zone id is not a valid UUID"})
		return
	}
	var zone models.Zone
	result := api.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "zone not found"})
		return
	}
	var peers []uuid.UUID
	api.db.Model(&models.Peer{}).Where("zone_id = ?", zone.ID).Pluck("id", &peers)
	zone.PeerList = peers
	c.JSON(http.StatusOK, zone)
}

// ListZonePeers lists all peers in a zone
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
func (api *API) ListPeersInZone(c *gin.Context) {
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "zone id is not a valid UUID"})
		return
	}
	var zone models.Zone
	result := api.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "zone not found"})
		return
	}
	peers := make([]models.Peer, 0)
	result = api.db.Where("zone_id = ?", zone.ID).Find(&peers)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "error fetching peers from db"})
		return
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", TotalCountHeader)
	c.Header(TotalCountHeader, strconv.Itoa(len(peers)))
	c.JSON(http.StatusOK, peers)
}

// GetZonePeers gets a peer in a zone
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
func (api *API) GetPeerInZone(c *gin.Context) {
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "zone id is not a valid UUID"})
		return
	}
	var zone models.Zone
	result := api.db.First(&zone, "id = ?", k.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "zone not found"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "zone id is not a valid UUID"})
		return
	}
	var peer models.Peer
	result = api.db.First(&peer, "id = ?", id.String())
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "peer not found"})
		return
	}
	c.JSON(http.StatusOK, peer)
}
