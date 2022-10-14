package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redhat-et/jaywalking/controltower/ipam"
	log "github.com/sirupsen/logrus"
)

// Prefix is used to unmarshall ipam lease information
type Prefix struct {
	Cidr string          // The Cidr of this prefix
	IPs  map[string]bool // The ips contained in this prefix
}

// PostZone creates a new zone via a REST call
func (ct *Controltower) PostZone(c *gin.Context) {
	var newZone Zone
	ctx := context.Background()
	// Call BindJSON to bind the received JSON to
	if err := c.BindJSON(&newZone); err != nil {
		return
	}
	if newZone.IpCidr == "" {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "the zone request did not contain a required CIDR prefix"})
		return
	}
	for _, zone := range ct.Zones {
		if zone.Name == newZone.Name {
			failMsg := fmt.Sprintf("%s zone already exists", newZone.Name)
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": failMsg})
			return
		}
	}
	log.Debugf("New zone request [ %s ] and ipam [ %s ] request", newZone.Name, newZone.IpCidr)

	zoneIpamSaveFile := fmt.Sprintf("%s.json", newZone.Name)
	// TODO: until we save control tower state between restarts, the ipam save file will be out of sync
	// new zones will delete the stale IPAM file on creation.
	// currently this will delete and overwrite an existing zone and ipam objects.
	if fileExists(zoneIpamSaveFile) {
		log.Warnf("ipam persistent storage file [ %s ] already exists on the control tower, deleting it", zoneIpamSaveFile)
		if err := deleteFile(zoneIpamSaveFile); err != nil {
			failMsg := fmt.Sprintf("unable to delete the ipam persistent storage file on the control tower [ %s ]: %v", zoneIpamSaveFile, err)
			c.IndentedJSON(http.StatusNotImplemented, gin.H{"message": failMsg})
		}
	}

	ipam, err := ipam.NewIPAM(ctx, zoneIpamSaveFile, newZone.IpCidr)
	if err != nil {
		failMsg := fmt.Sprintf("failed to acquire an ipam instance: %v", err)
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": failMsg})
	}
	newZone.ZoneIpam = *ipam
	if err := ipam.IpamSave(ctx); err != nil {
		log.Errorf("failed to save the ipam persistent storage file %v", err)
	}
	ct.Zones = append(ct.Zones, newZone)

	c.IndentedJSON(http.StatusCreated, newZone)
}

// GetZones responds with the list of all peers as JSON.
func (ct *Controltower) GetZones(c *gin.Context) {
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(ct.Zones)))
	c.JSON(http.StatusOK, ct.Zones)
}

// GetPeers responds with the list of all peers as JSON. TODO: Currently default zone only
func (ct *Controltower) GetPeers(c *gin.Context) {
	allNodes := make([]Peer, 0)
	for _, v := range ct.NodeMapDefault {
		allNodes = append(allNodes, v)
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(allNodes)))
	c.JSON(http.StatusOK, allNodes)
}

// GetPeerByKey locates the Peers whose PublicKey value matches the id
func (ct *Controltower) GetPeerByKey(c *gin.Context) {
	key := c.Param("key")
	for pubKey := range ct.NodeMapDefault {
		if pubKey == key {
			c.IndentedJSON(http.StatusOK, ct.NodeMapDefault[key])
			return
		}
	}

	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "peer not found"})
}

// TODO: this is hacky, should query the instance state instead, also file lock risk
// GetIpamLeases responds with the list of all peers as JSON.
func (ct *Controltower) GetIpamLeases(c *gin.Context) {
	zoneKey := c.Param("zone")
	zoneKeyFile := fmt.Sprintf("%s.json", zoneKey)
	var zoneLeases []Prefix
	var err error
	if fileExists(zoneKeyFile) {
		blueIpamState := fileToString(zoneKeyFile)
		zoneLeases, err = unmarshalIpamState(blueIpamState)
		if err != nil {
			log.Errorf("unable to unmarshall ipam leases: %v", err)
		}
	}

	c.JSON(http.StatusOK, zoneLeases)
}

/*
func publishAllPeersMessage(ctx context.Context, channel string, data []Peer) {
	id, msg := createAllPeerMessage(data)
	err := redisDB.Publish(ctx, channel, msg).Err()
	if err != nil {
		log.Errorf("sending %s message failed, %v\n", id, err)
		return
	}
	log.Printf("Published new message: %s\n", msg)
}
*/

func fileToString(file string) string {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		log.Errorf("unable to read the file [%s] %v\n", file, err)
		return ""
	}
	return string(fileContent)
}

func unmarshalIpamState(s string) ([]Prefix, error) {
	var msg []Prefix
	if err := json.Unmarshal([]byte(s), &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func fileExists(f string) bool {
	if _, err := os.Stat(f); err != nil {
		return false
	}
	return true
}

func deleteFile(f string) error {
	if err := os.Remove(f); err != nil {
		return err
	}
	return nil
}
