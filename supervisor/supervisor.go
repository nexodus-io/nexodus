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
	"github.com/go-redis/redis/v8"
	"github.com/redhat-et/jaywalking/supervisor/ipam"
	log "github.com/sirupsen/logrus"
)

var (
	redisDB       *redis.Client
	streamService *string
	streamPasswd  *string
)

const (
	zoneChannelBlue           = "zone-blue"
	zoneChannelRed            = "zone-red"
	ipPrefixBlue              = "10.20.1.0/20"
	ipPrefixRed               = "10.20.1.0/20"
	BlueIpamSaveFile          = "ipam-blue.json"
	RedIpamSaveFile           = "ipam-red.json"
	streamPort                = 6379
	restPort                  = "8080"
	healthcheckRequestChannel = "supervisor-healthcheck-request"
	healthcheckReplyChannel   = "supervisor-healthcheck-reply"
	healthcheckReplyMsg       = "supervisor-healthy"
	jwLogEnv                  = "JAYWALK_LOG_LEVEL"
)

func init() {
	streamService = flag.String("streamer-address", "", "streamer address")
	streamPasswd = flag.String("streamer-passwd", "", "streamer password")
	flag.Parse()
	// set the log level
	env := os.Getenv(jwLogEnv)
	if env == "debug" {
		log.SetLevel(log.DebugLevel)
	}
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
	ChildPrefix string `json:"ChildPrefix"`
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

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Unable to connect to the redis instance at %s: %v", sup.streamSocket, err)
	}

	readyCheckRepsonder(ctx, client)
	// Initilize ipam for zone blue
	ctxBlue := context.Background()
	supIpamBlue, err := ipam.NewIPAM(ctxBlue, BlueIpamSaveFile, ipPrefixBlue)
	if err != nil {
		log.Warnf("failed to acquire an ipam address %v\n", err)
	}

	pubBlue := newPubsub(newRedisClient(sup.streamSocket, sup.streamPass))
	subBlue := newPubsub(newRedisClient(sup.streamSocket, sup.streamPass))

	// channel for async messages from the zone subscription
	msgChanBlue := make(chan string)

	go func() {
		subBlue.subscribe(ctx, zoneChannelBlue, msgChanBlue)
		for {
			msg := <-msgChanBlue
			msgEvent := handleMsg(msg)
			switch msgEvent.Event {
			case registerNodeRequest:
				log.Debugf("Recieved registration request: %+v\n", msgEvent.Peer)
				if msgEvent.Peer.PublicKey != "" {
					nodeEvent := Peer{}
					var ip string
					// If this was a static address request
					if msgEvent.Peer.NodeAddress != "" {
						if err := ipam.ValidateIp(msgEvent.Peer.NodeAddress); err == nil {
							ip, err = supIpamBlue.RequestSpecificIP(ctxBlue, msgEvent.Peer.NodeAddress, ipPrefixBlue)
							if err != nil {
								log.Errorf("failed to assign the requested address, assigning an address from the pool %v\n", err)
								ip, err = supIpamBlue.RequestIP(ctxBlue, ipPrefixBlue)
								if err != nil {
									log.Errorf("failed to acquire an IPAM assigned address %v\n", err)
								}
							}
						}
					} else {
						ip, err = supIpamBlue.RequestIP(ctxBlue, ipPrefixBlue)
						if err != nil {
							log.Errorf("failed to acquire an IPAM assigned address %v\n", err)
						}
					}
					// allocate a child prefix if requested
					var childPrefix string
					if msgEvent.Peer.ChildPrefix != "" {
						childPrefix, err = supIpamBlue.RequestChildPrefix(ctxBlue, msgEvent.Peer.ChildPrefix)
						if err != nil {
							log.Errorf("%v\n", err)
						}
					}
					// save the ipam to persistent storage
					supIpamBlue.IpamSave(ctxBlue)
					// construct the new node
					nodeEvent = msgEvent.newNode(ip, childPrefix)
					log.Debugf("node allocated: %+v\n", nodeEvent)
					// delete the old k/v pair if one exists and replace it with the new registration data
					if _, ok := sup.NodeMapBlue[msgEvent.Peer.PublicKey]; ok {
						delete(sup.NodeMapBlue, msgEvent.Peer.PublicKey)
					}
					sup.NodeMapBlue[msgEvent.Peer.PublicKey] = nodeEvent
					// append all peers into the updated peer list to be published
					var peerList []Peer
					for pubKey, nodeElements := range sup.NodeMapBlue {
						log.Printf("NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s] NodeAddress [%s] Zone [%s] ChildPrefix [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs, nodeElements.NodeAddress, nodeElements.Zone, nodeElements.ChildPrefix)
						// append the new node to the updated peer listing
						peerList = append(peerList, nodeElements)
					}
					// publishPeers the latest peer list
					pubBlue.publishPeers(ctx, zoneChannelBlue, peerList)
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

	pubRed := newPubsub(newRedisClient(sup.streamSocket, sup.streamPass))
	subRed := newPubsub(newRedisClient(sup.streamSocket, sup.streamPass))

	// channel for async messages from the zone subscription
	msgChanRed := make(chan string)

	go func() {
		subRed.subscribe(ctx, zoneChannelRed, msgChanRed)
		for {
			msg := <-msgChanRed
			msgEvent := handleMsg(msg)
			switch msgEvent.Event {
			case registerNodeRequest:
				log.Debugf("Recieved registration request: %+v\n", msgEvent.Peer)
				if msgEvent.Peer.PublicKey != "" {
					nodeEvent := Peer{}
					var ip string
					// If this was a static address request
					if msgEvent.Peer.NodeAddress != "" {
						if err := ipam.ValidateIp(msgEvent.Peer.NodeAddress); err == nil {
							ip, err = supIpamRed.RequestSpecificIP(ctxRed, msgEvent.Peer.NodeAddress, ipPrefixRed)
							if err != nil {
								log.Errorf("failed to assign the requested address, assigning an address from the pool %v\n", err)
								ip, err = supIpamRed.RequestIP(ctxRed, ipPrefixRed)
								if err != nil {
									log.Errorf("failed to acquire an IPAM assigned address %v\n", err)
								}
							}
						}
					} else {
						ip, err = supIpamRed.RequestIP(ctxRed, ipPrefixRed)
						if err != nil {
							log.Errorf("failed to acquire an IPAM assigned address %v\n", err)
						}
					}

					// allocate a child prefix if requested
					var childPrefix string
					if msgEvent.Peer.ChildPrefix != "" {
						childPrefix, err = supIpamRed.RequestChildPrefix(ctxRed, msgEvent.Peer.ChildPrefix)
						if err != nil {
							log.Errorf("%v\n", err)
						}
					}
					// save the ipam to persistent storage
					supIpamRed.IpamSave(ctxRed)
					// construct the new node
					nodeEvent = msgEvent.newNode(ip, childPrefix)
					log.Debugf("node allocated: %+v\n", nodeEvent)
					// delete the old k/v pair if one exists and replace it with the new registration data
					if _, ok := sup.NodeMapRed[msgEvent.Peer.PublicKey]; ok {
						delete(sup.NodeMapRed, msgEvent.Peer.PublicKey)
					}
					sup.NodeMapRed[msgEvent.Peer.PublicKey] = nodeEvent
					// append all peers into the updated peer list to be published
					var peerList []Peer
					for pubKey, nodeElements := range sup.NodeMapRed {
						log.Printf("NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s] NodeAddress [%s] Zone [%s] ChildPrefix [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs, nodeElements.NodeAddress, nodeElements.Zone, nodeElements.ChildPrefix)
						// append the new node to the updated peer listing
						peerList = append(peerList, nodeElements)
					}
					// publishPeers the latest peer list
					pubRed.publishPeers(ctx, zoneChannelRed, peerList)
				}
			}
		}
	}()

	// Start the http router, this is blocking
	ginSocket := fmt.Sprintf("localhost:%s", restPort)
	sup.Router.Run(ginSocket)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	<-ch
}

func (msgEvent *MsgEvent) newNode(ipamIP, childPrefix string) Peer {
	peer := Peer{
		PublicKey:   msgEvent.Peer.PublicKey,
		EndpointIP:  msgEvent.Peer.EndpointIP,
		AllowedIPs:  ipamIP, // This will be a slice, NodeAddress will hold the /32
		Zone:        msgEvent.Peer.Zone,
		NodeAddress: ipamIP,
		ChildPrefix: childPrefix,
	}
	return peer
}

// handleMsg deals with streaming messages
func handleMsg(payload string) MsgEvent {
	var peer MsgEvent
	err := json.Unmarshal([]byte(payload), &peer)
	if err != nil {
		log.Debugf("HandleMsg unmarshall error: %v\n", err)
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
