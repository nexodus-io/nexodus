package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/redhat-et/jaywalking/supervisor/ipam"
	log "github.com/sirupsen/logrus"
)

var (
	redisDB       *redis.Client
	streamService *string
	streamPasswd  *string
)

const (
	zoneChannelBlue  = "zone-blue"
	zoneChannelRed   = "zone-red"
	ipPrefixBlue     = "10.10.1.0/20"
	ipPrefixRed      = "10.10.1.0/20"
	BlueIpamSaveFile = "ipam-blue.json"
	RedIpamSaveFile  = "ipam-red.json"
	streamPort       = 6379
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
	PublicKey   string `json:"PublicKey"`
	EndpointIP  string `json:"EndpointIP"`
	AllowedIPs  string `json:"AllowedIPs"`
	Zone        string `json:"Zone"`
	NodeAddress string `json:"NodeAddress"`
}

type ZoneConfig struct {
	Name        string
	Description string
	IpCidr      string
}

type MsgEvent struct {
	Event string
	Peer  Peer
}

// Supervisor data specific to the supervisor
type Supervisor struct {
	Router         *gin.Engine
	NodeMapRed     map[string]Peer
	NodeMapBlue    map[string]Peer
	ZoneConfigRed  map[string]ZoneConfig
	ZoneConfigBlue map[string]ZoneConfig
	stream         *redis.Client
	streamSocket   string
	streamPass     string
}

func initApp() *Supervisor {
	sup := new(Supervisor)
	sup.Router = gin.Default()
	sup.Router.GET("/peers", sup.GetPeers)                  // http://localhost:8080/peers
	sup.Router.GET("/peers/:key", sup.GetPeerByKey)         // http://localhost:8080/peers/pubkey
	sup.Router.GET("/zones", sup.GetZones)                  // http://localhost:8080/zones
	sup.Router.GET("/ipam/leases/:zone", sup.GetIpamLeases) // http://localhost:8080/zones
	sup.Router.POST("/peers", sup.PostPeers)                // TODO: not functioning
	sup.NodeMapBlue = make(map[string]Peer)
	sup.NodeMapRed = make(map[string]Peer)
	sup.ZoneConfigRed = make(map[string]ZoneConfig)
	sup.ZoneConfigBlue = make(map[string]ZoneConfig)
	sup.setZoneDetails(zoneChannelRed)
	sup.setZoneDetails(zoneChannelBlue)
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
	// Initilize ipam for zone blue
	ctxBlue := context.Background()
	supIpamBlue, err := ipam.NewIPAM(ctxBlue, BlueIpamSaveFile, ipPrefixBlue)
	if err != nil {
		log.Warnf("failed to acquire an ipam address %v\n", err)
	}
	supIpamBlue.IpamSave(ctxBlue)
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
					var ip string
					if msgEvent.Peer.NodeAddress != "" {
						// If this was a static address request
						if err := ipam.ValidateIp(msgEvent.Peer.NodeAddress); err == nil {
							ip, err = supIpamBlue.RequestSpecificIP(ctxBlue, msgEvent.Peer.NodeAddress, ipPrefixBlue)
							if err != nil {
								log.Errorf("failed to assign the requested address, assigning an address from the pool %v", err)
								ip, err = supIpamBlue.RequestIP(ctxBlue, ipPrefixBlue)
								if err != nil {
									log.Errorf("[ERROR] failed to acquire an IPAM assigned address %v", err)
								}
							}
						}
					} else {
						ip, err = supIpamBlue.RequestIP(ctxBlue, ipPrefixBlue)
						if err != nil {
							log.Errorf("[ERROR] failed to acquire an IPAM assigned address %v", err)
						}
					}
					supIpamBlue.IpamSave(ctxBlue)
					nodeEvent = msgEvent.newNode(ip)

					// delete the old k/v pair if one exists and replace it with the new registration data
					if _, ok := sup.NodeMapBlue[msgEvent.Peer.PublicKey]; ok {
						delete(sup.NodeMapBlue, msgEvent.Peer.PublicKey)
					}
					sup.NodeMapBlue[msgEvent.Peer.PublicKey] = nodeEvent
					var peerList []Peer
					for pubKey, nodeElements := range sup.NodeMapBlue {
						fmt.Printf("NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s] NodeAddress [%s] Zone [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs, nodeElements.NodeAddress, nodeElements.Zone)
						peerList = append(peerList, nodeElements)
					}
					pubBlue.publish(zoneChannelBlue, peerList)
				}
			}
		}
	}()

	// Initilize ipam for zone red
	// depending on how ctx background is being used a second instance may be unnecessary for multi-tenancy
	ctxRed := context.Background()
	supIpamRed, err := ipam.NewIPAM(ctxRed, RedIpamSaveFile, ipPrefixRed)
	if err != nil {
		log.Errorf("failed to acquire an ipam address %v\n", err)
	}
	supIpamRed.IpamSave(ctxRed)

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
					var ip string
					if msgEvent.Peer.NodeAddress != "" {
						// If this was a static address request
						if err := ipam.ValidateIp(msgEvent.Peer.NodeAddress); err == nil {
							ip, err = supIpamRed.RequestSpecificIP(ctxRed, msgEvent.Peer.NodeAddress, ipPrefixRed)
							if err != nil {
								log.Errorf("failed to assign the requested address, assigning an address from the pool %v", err)
								ip, err = supIpamRed.RequestIP(ctxRed, ipPrefixRed)
								if err != nil {
									log.Errorf("failed to acquire an IPAM assigned address %v", err)
								}
							}
						}
					} else {
						ip, err = supIpamRed.RequestIP(ctxRed, ipPrefixRed)
						if err != nil {
							log.Errorf("failed to acquire an IPAM assigned address %v", err)
						}
					}
					supIpamRed.IpamSave(ctxRed)
					nodeEvent = msgEvent.newNode(ip)
					// delete the old k/v pair if one exists and replace it with the new registration data
					if _, ok := sup.NodeMapRed[msgEvent.Peer.PublicKey]; ok {
						delete(sup.NodeMapRed, msgEvent.Peer.PublicKey)
					}
					sup.NodeMapRed[msgEvent.Peer.PublicKey] = nodeEvent
					var peerList []Peer
					for pubKey, nodeElements := range sup.NodeMapRed {
						fmt.Printf("NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s] NodeAddress [%s] Zone [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs, nodeElements.NodeAddress, nodeElements.Zone)
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
		PublicKey:   msgEvent.Peer.PublicKey,
		EndpointIP:  msgEvent.Peer.EndpointIP,
		AllowedIPs:  ipamIP, // This will be a slice, NodeAddress will hold the /32
		Zone:        msgEvent.Peer.Zone,
		NodeAddress: ipamIP,
	}
	return peer
}

// handleMsg deals with streaming messages
func handleMsg(payload string) MsgEvent {
	var peer MsgEvent
	err := json.Unmarshal([]byte(payload), &peer)
	if err != nil {
		log.Printf("HandleMsg unmarshall error: %v\n", err)
		return peer
	}
	return peer
}

// setZoneDetails set general zone attributes
func (sup *Supervisor) setZoneDetails(zone string) {
	if zone == zoneChannelBlue {
		zoneConfBlue := ZoneConfig{
			Name:        zone,
			Description: "Tenancy Zone Blue",
			IpCidr:      ipPrefixBlue,
		}
		sup.ZoneConfigRed[zoneChannelBlue] = zoneConfBlue
	}
	if zone == zoneChannelRed {
		zoneConfRed := ZoneConfig{
			Name:        zone,
			Description: "Tenancy Zone Red",
			IpCidr:      ipPrefixRed,
		}
		sup.ZoneConfigRed[zoneChannelRed] = zoneConfRed
	}
}
