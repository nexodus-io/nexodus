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

type Supervisor struct {
	Router       *gin.Engine
	nodeMapRed   map[string]Peer
	nodeMapBlue  map[string]Peer
	stream       *redis.Client
	streamSocket string
	streamPass   string
}

func initApp() *Supervisor {
	sup := new(Supervisor)
	sup.Router = gin.Default()
	sup.Router.GET("/peers", sup.getPeers)          // curl http://localhost:8080/peers
	sup.Router.GET("/peers/:key", sup.getPeerByKey) // curl http://localhost:8080/peers/pubkey
	sup.Router.POST("/peers", sup.postPeers)        // TODO: not functioning
	sup.nodeMapBlue = make(map[string]Peer)
	sup.nodeMapRed = make(map[string]Peer)
	sup.streamSocket = fmt.Sprintf("%s:%d", *streamService, streamPort)
	sup.streamPass = *streamPasswd

	return sup
}

func main() {

	sup := initApp()

	client := newRedisClient(sup.streamSocket, sup.streamPass)
	defer client.Close()

	_, err := client.Ping().Result()
	if err != nil {
		log.Fatalf("Unable to connect to the redis instance at %s: %v", sup.streamSocket, err)
	}

	pubBlue := newPubsub(newRedisClient(sup.streamSocket, sup.streamPass))
	subBlue := newPubsub(newRedisClient(sup.streamSocket, sup.streamPass))

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
					if _, ok := sup.nodeMapBlue[msgEvent.Peer.PublicKey]; ok {
						nodeEvent = msgEvent.newNode(sup.nodeMapBlue[msgEvent.Peer.PublicKey].AllowedIPs)
					} else {
						// if this is a new node, assign a new ipam address
						lameIPAM := len(sup.nodeMapBlue) + 1
						ipamIP := fmt.Sprintf("%s.%d/32", ipPrefixBlue, lameIPAM)
						nodeEvent = msgEvent.newNode(ipamIP)
					}
					// delete the old k/v pair if one exists and replace it with the new registration data
					if _, ok := sup.nodeMapBlue[msgEvent.Peer.PublicKey]; ok {
						delete(sup.nodeMapBlue, msgEvent.Peer.PublicKey)
					}
					sup.nodeMapBlue[msgEvent.Peer.PublicKey] = nodeEvent
					var peerList []Peer
					for pubKey, nodeElements := range sup.nodeMapBlue {
						fmt.Printf("[INFO] NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs)
						peerList = append(peerList, nodeElements)
					}
					pubBlue.publish(zoneChannelBlue, peerList)
				}
			}
		}
	}()

	pubRed := newPubsub(newRedisClient(sup.streamSocket, sup.streamPass))
	subRed := newPubsub(newRedisClient(sup.streamSocket, sup.streamPass))

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
					if _, ok := sup.nodeMapRed[msgEvent.Peer.PublicKey]; ok {
						nodeEvent = msgEvent.newNode(sup.nodeMapRed[msgEvent.Peer.PublicKey].AllowedIPs)
					} else {
						// if this is a new node, assign a new ipam address
						lameIPAM := len(sup.nodeMapRed) + 1
						ipamIP := fmt.Sprintf("%s.%d/32", ipPrefixRed, lameIPAM)
						nodeEvent = msgEvent.newNode(ipamIP)
					}
					// delete the old k/v pair if one exists and replace it
					if _, ok := sup.nodeMapRed[msgEvent.Peer.PublicKey]; ok {
						delete(sup.nodeMapRed, msgEvent.Peer.PublicKey)
					}
					sup.nodeMapRed[msgEvent.Peer.PublicKey] = nodeEvent

					var peerList []Peer
					for pubKey, nodeElements := range sup.nodeMapRed {
						fmt.Printf("[INFO] NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs)
						peerList = append(peerList, nodeElements)
					}
					pubRed.publish(zoneChannelRed, peerList)
				}
			}
		}
	}()
	// Start the http router
	sup.Router.Run("localhost:8080")

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
