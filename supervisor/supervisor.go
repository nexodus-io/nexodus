package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
)

var (
	redisDB       *redis.Client
	streamService *string
	streamPasswd  *string
)

const (
	zoneChannelBlue = "zone-blue"
	zoneChannelRed  = "zone-red"
	streamPort      = 6379
	ipPrefixBlue    = "10.10.1"
	ipPrefixRed     = "10.20.1"
)

func init() {
	streamService = flag.String("streamer-address", "", "streamer address")
	streamPasswd = flag.String("streamer-passwd", "", "streamer password")
	flag.Parse()

	streamSocket := fmt.Sprintf("%s:%d", *streamService, streamPort)
	redisDB = redis.NewClient(&redis.Options{
		Addr:     streamSocket,
		Password: *streamPasswd,
	})

	// Start the gin router
	router := gin.Default()
	router.GET("/peers", getPeers)
	router.GET("/peers/:id", getPeerByKey)
	router.POST("/peers", postPeers)
	go router.Run("localhost:8080")

}

// Message Events
const (
	registerNodeRequest = "register-node-request"
	registerNodeReply   = "register-node-reply"
)

// Peer represents data about a Peer's record.
type Peer struct {
	PublicKey  string `json:"PublicKey"`
	EndpointIP string `json:"EndpointIP"`
	AllowedIPs string `json:"AllowedIPs"`
}

type MsgEvent struct {
	Event string
	Peer  Peer
}

func main() {

	defer redisDB.Close()
	pubsub := redisDB.Subscribe(zoneChannelBlue)
	defer pubsub.Close()

	fmt.Printf("[INFO] Subscribed to channel: %s\n", zoneChannelBlue)

	var nodeSate = make(map[string]Peer)
	for {
		msg, err := pubsub.ReceiveMessage()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		switch msg.Channel {
		case zoneChannelBlue:
			incomingMsg := handleMsg(msg.Payload)
			switch incomingMsg.Event {
			case registerNodeRequest:

				if incomingMsg.Peer.PublicKey != "" {
					lameIPAM := len(nodeSate) + 1
					wireguardIP := fmt.Sprintf("%s.%d/32", ipPrefixBlue, lameIPAM)
					newNode := Peer{
						PublicKey:  incomingMsg.Peer.PublicKey,
						EndpointIP: incomingMsg.Peer.EndpointIP,
						AllowedIPs: wireguardIP,
					}
					nodeSate[incomingMsg.Peer.PublicKey] = newNode
					var peerList []Peer
					for pubKey, nodeElements := range nodeSate {
						fmt.Printf("NodeState -> PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs)
						peerList = append(peerList, nodeElements)
					}
					publishAllPeersMessage(zoneChannelBlue, peerList)

				}
			}
		case zoneChannelRed:
			incomingMsg := handleMsg(msg.Payload)
			switch incomingMsg.Event {
			case registerNodeRequest:

				if incomingMsg.Peer.PublicKey != "" {
					lameIPAM := len(nodeSate) + 1
					wireguardIP := fmt.Sprintf("%s.%d/32", ipPrefixRed, lameIPAM)
					newNode := Peer{
						PublicKey:  incomingMsg.Peer.PublicKey,
						EndpointIP: incomingMsg.Peer.EndpointIP,
						AllowedIPs: wireguardIP,
					}
					nodeSate[incomingMsg.Peer.PublicKey] = newNode
					var peerList []Peer
					for pubKey, nodeElements := range nodeSate {
						fmt.Printf("NodeState -> PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs)
						peerList = append(peerList, nodeElements)
					}
					publishAllPeersMessage(zoneChannelBlue, peerList)

				}
			}
		}
		// TODO: Unnecessary but temporarily useful for debugging
		time.Sleep(1 * time.Second)
	}
}

// handleMsg deals with streaming messages
func handleMsg(payload string) MsgEvent {
	var peer MsgEvent
	err := json.Unmarshal([]byte(payload), &peer)
	if err != nil {
		log.Printf("[ERROR] UnmarshalMessage: %v\n", err)
		return peer
	}
	return peer
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

func createAllPeerMessage(postData []Peer) (string, string) {
	id := uuid.NewString()
	msg, _ := json.Marshal(postData)
	return id, string(msg)
}

func UnmarshalMessage(s string) (*MsgEvent, error) {
	var msg MsgEvent
	if err := json.Unmarshal([]byte(s), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func NewMessage(event, pubKey, privateKey, allowedIPs string) (string, string) {
	id := uuid.NewString()
	msg := MsgEvent{}
	msg.Event = event
	peer := Peer{
		PublicKey:  pubKey,
		EndpointIP: privateKey,
		AllowedIPs: allowedIPs,
	}
	msg.Peer = peer
	jMsg, _ := json.Marshal(&msg)
	return id, string(jMsg)
}

func PublishMessage(channel, msg string) {
	err := redisDB.Publish(channel, msg).Err()
	if err != nil {
		log.Printf("[ERROR] sending message, %v\n", err)
		return
	}
	fmt.Printf("[INFO] SENT ---> %s\n", msg)
}
