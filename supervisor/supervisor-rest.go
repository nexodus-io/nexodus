package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// GetPeers responds with the list of all peers as JSON.
func (sup *Supervisor) GetPeers(c *gin.Context) {
	allNodes := make([]Peer, 0)
	for _, v := range sup.NodeMapBlue {
		allNodes = append(allNodes, v)
	}
	for _, v := range sup.NodeMapRed {
		allNodes = append(allNodes, v)
	}

	c.JSON(http.StatusOK, allNodes)
}

// GetPeerByKey locates the Peers whose PublicKey value matches the id
func (sup *Supervisor) GetPeerByKey(c *gin.Context) {
	key := c.Param("key")
	for pubKey, _ := range sup.NodeMapBlue {
		if pubKey == key {
			c.IndentedJSON(http.StatusOK, sup.NodeMapBlue[key])
			return
		}
	}
	for pubKey, _ := range sup.NodeMapRed {
		if pubKey == key {
			c.IndentedJSON(http.StatusOK, sup.NodeMapRed[key])
			return
		}
	}

	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "peer not found"})
}

// GetZones responds with the list of all peers as JSON.
func (sup *Supervisor) GetZones(c *gin.Context) {
	zones := make([]ZoneConfig, 0)
	for _, v := range sup.ZoneConfigBlue {
		zones = append(zones, v)
	}
	for _, v := range sup.ZoneConfigRed {
		zones = append(zones, v)
	}

	c.JSON(http.StatusOK, zones)
}

// TODO: this is hacky, should query the instance state instead, also file lock risk
// GetIpamLeases responds with the list of all peers as JSON.
func (sup *Supervisor) GetIpamLeases(c *gin.Context) {
	zoneKey := c.Param("zone")
	var zoneLeases []Prefix
	var err error
	if zoneKey == zoneChannelBlue {
		if fileExists(BlueIpamSaveFile) {
			blueIpamState := fileToString(BlueIpamSaveFile)
			zoneLeases, err = unmarshalIpamState(blueIpamState)
			if err != nil {
				fmt.Printf("[ERROR] unable to unmarshall ipam leases: %v", err)
			}
		}
	}
	if zoneKey == zoneChannelRed {
		if fileExists(RedIpamSaveFile) {
			redIpamState := fileToString(RedIpamSaveFile)
			zoneLeases, err = unmarshalIpamState(redIpamState)
			if err != nil {
				fmt.Printf("[ERROR] unable to unmarshall ipam leases: %v", err)
			}
		}
	}

	c.JSON(http.StatusOK, zoneLeases)
}

// PostPeers adds a Peers from JSON received in the request body.
func (sup *Supervisor) PostPeers(c *gin.Context) {
	var newPeer []Peer
	// Call BindJSON to bind the received JSON to
	if err := c.BindJSON(&newPeer); err != nil {
		return
	}
	// TODO: add single node config
	// Add the new Peers to the slice.
	//peers = append(peers, newPeer)
	c.IndentedJSON(http.StatusCreated, newPeer)
	// TODO: broken atm
	// publishAllPeersMessage(ctx, zoneChannelBlue, newPeer)
}

func publishAllPeersMessage(ctx context.Context, channel string, data []Peer) {
	id, msg := createAllPeerMessage(data)
	err := redisDB.Publish(ctx, channel, msg).Err()
	if err != nil {
		log.Errorf("sending %s message failed, %v\n", id, err)
		return
	}
	log.Printf("Published new message: %s\n", msg)
}

func fileToString(file string) string {
	fileContent, err := ioutil.ReadFile(file)
	if err != nil {
		log.Errorf("unable to read the file [%s] %v\n", file, err)
		return ""
	}
	return string(fileContent)
}

// Prefix is used to unmarshall ipam lease information
type Prefix struct {
	Cidr string          // The Cidr of this prefix
	IPs  map[string]bool // The ips contained in this prefix
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
