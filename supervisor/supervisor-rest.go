package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	peers []Peer
)

// getPeers responds with the list of all peers as JSON.
func getPeers(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, peers)
}

// postPeers adds a Peers from JSON received in the request body.
func postPeers(c *gin.Context) {
	var newPeer []Peer

	// Call BindJSON to bind the received JSON to
	if err := c.BindJSON(&newPeer); err != nil {
		return
	}

	// TODO: add single node config
	// Add the new Peers to the slice.
	//peers = append(peers, newPeer)
	c.IndentedJSON(http.StatusCreated, newPeer)

	log.Println("[INFO] New node added, pushing changes to the mesh..")
	publishAllPeersMessage(zoneChannelBlue, newPeer)

}

// getPeerByKey locates the Peers whose PublicKey value matches the id
// parameter sent by the client, then returns that Peers as a response.
func getPeerByKey(c *gin.Context) {
	id := c.Param("id")

	// Loop through the list of peers, looking for
	// a Peers whose PublicKey value matches the parameter.
	for _, a := range peers {
		if a.PublicKey == id {
			c.IndentedJSON(http.StatusOK, a)
			return
		}
	}
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "Peers not found"})
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
