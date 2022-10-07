package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// getPeers responds with the list of all peers as JSON.
func (sup *Supervisor) getPeers(c *gin.Context) {
	allNodes := make([]Peer, 0)
	for _, v := range sup.nodeMapBlue {
		allNodes = append(allNodes, v)
	}
	for _, v := range sup.nodeMapRed {
		allNodes = append(allNodes, v)
	}

	c.JSON(http.StatusOK, allNodes)
}

// getPeerByKey locates the Peers whose PublicKey value matches the id
func (sup *Supervisor) getPeerByKey(c *gin.Context) {
	key := c.Param("key")
	for pubKey, _ := range sup.nodeMapBlue {
		if pubKey == key {
			c.IndentedJSON(http.StatusOK, sup.nodeMapBlue[key])
			return
		}
	}
	for pubKey, _ := range sup.nodeMapRed {
		if pubKey == key {
			c.IndentedJSON(http.StatusOK, sup.nodeMapRed[key])
			return
		}
	}

	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "peer not found"})
}

// postPeers adds a Peers from JSON received in the request body.
func (sup *Supervisor) postPeers(c *gin.Context) {
	var newPeer []Peer
	// Call BindJSON to bind the received JSON to
	if err := c.BindJSON(&newPeer); err != nil {
		return
	}
	// TODO: add single node config
	// Add the new Peers to the slice.
	//peers = append(peers, newPeer)
	c.IndentedJSON(http.StatusCreated, newPeer)

	publishAllPeersMessage(zoneChannelBlue, newPeer)
}

func publishAllPeersMessage(channel string, data []Peer) {
	id, msg := createAllPeerMessage(data)
	err := redisDB.Publish(channel, msg).Err()
	if err != nil {
		log.Printf("[ERROR] sending %s message failed, %v\n", id, err)
		return
	}
	log.Printf("[INFO] Published new message: %s\n", msg)
}
