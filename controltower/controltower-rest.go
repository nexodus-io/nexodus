package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// PostZone creates a new zone via a REST call
func (ct *Controltower) PostZone(c *gin.Context) {
	var tmp ZoneConfig
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&tmp); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if tmp.IpCidr == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the zone request did not contain a required CIDR prefix"})
		return
	}
	if tmp.Name == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the zone request did not contain a required name"})
		return
	}

	// Create the zone
	newZone, err := NewZone(uuid.New(), tmp.Name, tmp.Description, tmp.IpCidr)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "unable to create zone"})
		return
	}

	// TODO: From this point on Zone should get cleaned up if we fail
	for _, zone := range ct.Zones {
		if zone.Name == newZone.Name {
			failMsg := fmt.Sprintf("%s zone already exists", newZone.Name)
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": failMsg})
			return
		}
	}
	log.Debugf("New zone request [ %s ] and ipam [ %s ] request", newZone.Name, newZone.IpCidr)
	ct.Zones[newZone.ID] = newZone
	c.IndentedJSON(http.StatusCreated, newZone)
}

// GetZones responds with the list of all peers as JSON.
func (ct *Controltower) GetZones(c *gin.Context) {
	allZones := make([]*Zone, 0)
	for _, z := range ct.Zones {
		allZones = append(allZones, z)
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(allZones)))
	c.JSON(http.StatusOK, allZones)
}

// GetZone responds with a single zone
func (ct *Controltower) GetZone(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if value, ok := ct.Zones[k]; ok {
		c.JSON(http.StatusOK, value)
	} else {
		c.Status(http.StatusNotFound)
	}
}

// GetPeers responds with the list of all peers as JSON. TODO: Currently default zone only
func (ct *Controltower) GetPeers(c *gin.Context) {
	pubKey := c.Query("public-key")
	var allPeers []*Peer
	if pubKey != "" {
		allPeers = ct.Peers.ListByPubKey(pubKey)
	} else {
		allPeers = ct.Peers.List()
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(allPeers)))
	c.JSON(http.StatusOK, allPeers)
}

// GetPeerByKey locates the Peers
func (ct *Controltower) GetPeer(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	v, err := ct.Peers.Get(k)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, v)
}
