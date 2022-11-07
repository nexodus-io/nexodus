package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ListPeers lists all peers
// @Summary      List Peers
// @Description  Lists all peers
// @Tags         Peers
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []models.Peer
// @Failure		 401  {object}  models.ApiError
// @Failure		 500  {object}  models.ApiError
// @Router       /peers [get]
func (api *API) ListPeers(c *gin.Context) {
	peers := make([]models.Peer, 0)
	result := api.db.Debug().Scopes(FilterAndPaginate(&models.Peer{}, c)).Find(&peers)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}
	c.JSON(http.StatusOK, peers)
}

// GetPeers gets a peer
// @Summary      Get Peer
// @Description  Gets a peer
// @Tags         Peers
// @Accepts		 json
// @Produce      json
// @Param		 peer_id path   string true "Zone ID"
// @Success      200  {object}  []models.Peer
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure      404  {object}  models.ApiError
// @Failure		 500  {object}  models.ApiError
// @Router       /peers/{peer_id} [get]
func (api *API) GetPeers(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "peer id is not a valid UUID"})
		return
	}
	var peer models.Peer
	result := api.db.First(&peer, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "peer not found"})
		return
	}
	c.JSON(http.StatusOK, peer)
}

// CreatePeerInZone adds a new peer in a zone
// @Summary      Add Peer
// @Description  Adds a new Peer in this Zone
// @Tags         Peers
// @Accepts		 json
// @Produce      json
// @Param		 zone_id path   string true "Zone ID"
// @Param		 peer    body   models.AddPeer true "Add Peer"
// @Success      201  {object}  models.Peer
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure		 403  {object}  models.ApiError
// @Failure      404  {object}  models.ApiError
// @Failure      409  {object}  models.Peer
// @Failure		 500  {object}  models.ApiError
// @Router       /zones/{zone_id}/peers [post]
func (api *API) CreatePeerInZone(c *gin.Context) {
	ctx := c.Request.Context()
	k, err := uuid.Parse(c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "zone id is not a valid UUID"})
		return
	}

	var zone models.Zone
	if res := api.db.Preload("Peers").First(&zone, "id = ?", k.String()); res.Error != nil {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "zone not found"})
		return
	}

	var request models.AddPeer
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: err.Error()})
		return
	}

	tx := api.db.Begin()

	var device models.Device
	if res := tx.First(&device, "id = ?", request.DeviceID); res.Error != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}

	var user models.User
	if res := tx.First(&user, "id = ?", device.UserID); res.Error != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}

	if user.ZoneID != zone.ID {
		c.JSON(http.StatusForbidden, models.ApiError{Error: fmt.Sprintf("user id %s is not part of zone %s", user.ID, zone.ID)})
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
	var peer *models.Peer
	for _, p := range zone.Peers {
		if p.DeviceID == device.ID {
			found = true
			peer = p
			break
		}
	}
	if found {
		peer.ReflexiveIPv4 = request.ReflexiveIPv4
		peer.EndpointIP = request.EndpointIP

		if request.NodeAddress != peer.NodeAddress {
			var ip string
			if request.NodeAddress != "" {
				var err error
				ip, err = api.ipam.AssignSpecificNodeAddress(ctx, ipamPrefix, request.NodeAddress)
				if err != nil {
					tx.Rollback()
					log.Error(err)
					c.JSON(http.StatusInternalServerError, models.ApiError{Error: "ipam error"})
					return
				}
			} else {
				if peer.NodeAddress == "" {
					c.JSON(http.StatusBadRequest, models.ApiError{Error: "peer does not have a node address assigned in the peer table and did not request a specifc address"})
					return
				}
				ip = peer.NodeAddress
			}
			peer.NodeAddress = ip
			peer.AllowedIPs = ip
		}

		if request.ChildPrefix != peer.ChildPrefix {
			if err := api.ipam.AssignPrefix(ctx, request.ChildPrefix); err != nil {
				tx.Rollback()
				log.Error(err)
				c.JSON(http.StatusInternalServerError, models.ApiError{Error: "ipam error"})
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
			ip, err = api.ipam.AssignSpecificNodeAddress(ctx, ipamPrefix, request.NodeAddress)
			if err != nil {
				tx.Rollback()
				log.Error(err)
				c.JSON(http.StatusInternalServerError, models.ApiError{Error: "ipam error"})
				return
			}
		} else {
			var err error
			ip, err = api.ipam.AssignFromPool(ctx, ipamPrefix)
			if err != nil {
				tx.Rollback()
				log.Error(err)
				c.JSON(http.StatusInternalServerError, models.ApiError{Error: "ipam error"})
				return
			}
		}
		// allocate a child prefix if requested
		if request.ChildPrefix != "" {
			if err := api.ipam.AssignPrefix(ctx, request.ChildPrefix); err != nil {
				tx.Rollback()
				log.Error(err)
				c.JSON(http.StatusInternalServerError, models.ApiError{Error: "ipam error"})
				return
			}
		}
		peer = &models.Peer{
			DeviceID:    device.ID,
			ZoneID:      zone.ID,
			EndpointIP:  request.EndpointIP,
			AllowedIPs:  ip,
			NodeAddress: ip,
			ChildPrefix: request.ChildPrefix,
			ZonePrefix:  ipamPrefix,
			HubZone:     hubZone,
			HubRouter:   hubRouter,
			ReflexiveIPv4: request.ReflexiveIPv4,
		}
		tx.Create(peer)
		zone.Peers = append(zone.Peers, peer)
		tx.Save(&zone)
		device.Peers = append(device.Peers, peer)
		tx.Save(&device)
	}
	tx.Save(&peer)

	if err := tx.Commit(); err.Error != nil {
		tx.Rollback()
		log.Error(err.Error)
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}

	c.JSON(http.StatusCreated, peer)
}
