package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	ipPrefixRed     = "10.10.1"
)

func init() {
	streamService = flag.String("streamer-address", "", "streamer address")
	streamPasswd = flag.String("streamer-passwd", "", "streamer password")
	flag.Parse()

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
	Zone       string `json:"Zone"`
}

type MsgEvent struct {
	Event string
	Peer  Peer
}

func main() {
	streamerSocket := fmt.Sprintf("%s:%d", *streamService, streamPort)

	client := newRedisClient(streamerSocket, *streamPasswd)
	defer client.Close()

	_, err := client.Ping().Result()
	if err != nil {
		log.Fatalf("Unable to connect to the redis instance at %s: %v", streamerSocket, err)
	}

	pubBlue := newPubsub(newRedisClient(streamerSocket, *streamPasswd))
	subBlue := newPubsub(newRedisClient(streamerSocket, *streamPasswd))
	var nodeStateBlue = make(map[string]Peer)
	msgChanBlue := make(chan string)
	go func() {
		subBlue.subscribe(zoneChannelBlue, msgChanBlue)
		for {
			msg := <-msgChanBlue
			msgEvent := handleMsg(msg)
			switch msgEvent.Event {
			case registerNodeRequest:
				if msgEvent.Peer.PublicKey != "" {
					nodeEvent := Peer{}
					// if the node already exists, preserve it's wireguard IP,
					// delete the peer and add in the latest state
					if _, ok := nodeStateBlue[msgEvent.Peer.PublicKey]; ok {
						nodeEvent = msgEvent.newNode(nodeStateBlue[msgEvent.Peer.PublicKey].AllowedIPs)
					} else {
						// if this is a new node, assign a new ipam address
						lameIPAM := len(nodeStateBlue) + 1
						ipamIP := fmt.Sprintf("%s.%d/32", ipPrefixBlue, lameIPAM)
						nodeEvent = msgEvent.newNode(ipamIP)
					}
					// delete the old k/v pair if one exists and replace it with the new registration data
					if _, ok := nodeStateBlue[msgEvent.Peer.PublicKey]; ok {
						delete(nodeStateBlue, msgEvent.Peer.PublicKey)
					}
					nodeStateBlue[msgEvent.Peer.PublicKey] = nodeEvent
					var peerList []Peer
					for pubKey, nodeElements := range nodeStateBlue {
						fmt.Printf("[INFO] NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs)
						peerList = append(peerList, nodeElements)
					}
					pubBlue.publish(zoneChannelBlue, peerList)
				}
			}
		}
	}()

	pubRed := newPubsub(newRedisClient(streamerSocket, *streamPasswd))
	subRed := newPubsub(newRedisClient(streamerSocket, *streamPasswd))
	var nodeStateRed = make(map[string]Peer)
	msgChanRed := make(chan string)
	go func() {
		subRed.subscribe(zoneChannelRed, msgChanRed)
		for {
			msg := <-msgChanRed
			msgEvent := handleMsg(msg)
			switch msgEvent.Event {
			case registerNodeRequest:
				if msgEvent.Peer.PublicKey != "" {
					nodeEvent := Peer{}
					// if the node already exists, preserve it's wireguard IP,
					// delete the peer and add in the latest state
					if _, ok := nodeStateRed[msgEvent.Peer.PublicKey]; ok {
						nodeEvent = msgEvent.newNode(nodeStateRed[msgEvent.Peer.PublicKey].AllowedIPs)
					} else {
						// if this is a new node, assign a new ipam address
						lameIPAM := len(nodeStateRed) + 1
						ipamIP := fmt.Sprintf("%s.%d/32", ipPrefixRed, lameIPAM)
						nodeEvent = msgEvent.newNode(ipamIP)
					}
					// delete the old k/v pair if one exists and replace it
					if _, ok := nodeStateRed[msgEvent.Peer.PublicKey]; ok {
						delete(nodeStateRed, msgEvent.Peer.PublicKey)
					}
					nodeStateRed[msgEvent.Peer.PublicKey] = nodeEvent

					var peerList []Peer
					for pubKey, nodeElements := range nodeStateRed {
						fmt.Printf("[INFO] NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs)
						peerList = append(peerList, nodeElements)
					}
					pubRed.publish(zoneChannelRed, peerList)
				}
			}
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	<-ch
}

func (msgEvent *MsgEvent) newNode(ipamIP string) Peer {
	peer := Peer{
		PublicKey:  msgEvent.Peer.PublicKey,
		EndpointIP: msgEvent.Peer.EndpointIP,
		AllowedIPs: ipamIP,
		Zone:       msgEvent.Peer.Zone,
	}
	return peer
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

// pubSub redis pubsub struct
type pubSub struct {
	client *redis.Client
}

// dispose dispose pubsub instance
func (pubsub *pubSub) dispose() {
	pubsub.client.Close()
}

// newPubsub create pubsub instance
func newPubsub(client *redis.Client) *pubSub {
	return &pubSub{client: client}
}

// publish publish a message into the channel
func (pubsub *pubSub) publish(channel string, data []Peer) (int64, error) {
	_, msg := createAllPeerMessage(data)
	log.Printf("[INFO] Published new message: %s\n", msg)
	return pubsub.client.Publish(channel, msg).Result()
}

// subscribe subscribe a redis channel to receive message
func (pubsub *pubSub) subscribe(channel string, msg chan string) {
	sub := pubsub.client.Subscribe(channel)
	go func() {
		for {
			outPut, _ := sub.ReceiveMessage()
			msg <- outPut.Payload
		}
	}()
}

// newRedisClient creates a redis client instance
func newRedisClient(streamerSocket, streamPasswd string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     streamerSocket,
		Password: streamPasswd,
		DB:       0,
	})
}

func createAllPeerMessage(postData []Peer) (string, string) {
	id := uuid.NewString()
	msg, _ := json.Marshal(postData)
	return id, string(msg)
}

func unmarshalMessage(s string) (*MsgEvent, error) {
	var msg MsgEvent
	if err := json.Unmarshal([]byte(s), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
