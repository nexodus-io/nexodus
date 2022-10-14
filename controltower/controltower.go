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
	"github.com/redhat-et/jaywalking/controltower/ipam"
	log "github.com/sirupsen/logrus"
)

var (
	redisDB       *redis.Client
	streamService *string
	streamPasswd  *string
)

const (
	zoneChannelController     = "controller"
	zoneChannelDefault        = "default"
	ipPrefixDefault           = "10.200.1.0/20"
	DefaultIpamSaveFile       = "default-ipam.json"
	DefaultZoneName           = "default"
	streamPort                = 6379
	restPort                  = "8080"
	healthcheckRequestChannel = "controltower-healthcheck-request"
	healthcheckReplyChannel   = "controltower-healthcheck-reply"
	healthcheckReplyMsg       = "controltower-healthy"
	ctLogEnv                  = "CONTROLTOWER_LOG_LEVEL"
)

// Message Events
const (
	registerNodeRequest = "register-node-request"
	registerNodeReply   = "register-node-reply"
)

func init() {
	streamService = flag.String("streamer-address", "", "streamer address")
	streamPasswd = flag.String("streamer-passwd", "", "streamer password")
	flag.Parse()
	// set the log level
	env := os.Getenv(ctLogEnv)
	if env == "debug" {
		log.SetLevel(log.DebugLevel)
	}
}

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

type Zone struct {
	NodeMap     map[string]Peer
	Name        string `json:"Name"`
	Description string `json:"Description"`
	IpCidr      string `json:"CIDR"`
	ZoneIpam    ipam.AirliftIpam
}

// Control tower specific data
type Controltower struct {
	Router            *gin.Engine
	Zones             []Zone
	NodeMapDefault    map[string]Peer
	ZoneConfigDefault map[string]ZoneConfig
	stream            *redis.Client
	streamSocket      string
	streamPass        string
}

func initApp() *Controltower {
	ct := new(Controltower)
	ct.Router = gin.Default()
	ct.Router.GET("/peers", ct.GetPeers)                  // http://localhost:8080/peers TODO: only functioning for zone:default atm
	ct.Router.GET("/peers/:key", ct.GetPeerByKey)         // http://localhost:8080/peers/pubkey
	ct.Router.GET("/ipam/leases/:zone", ct.GetIpamLeases) // http://localhost:8080/leases/:zone-name
	ct.Router.GET("/zones", ct.GetZones)                  // http://localhost:8080/zones
	ct.Router.POST("/zone", ct.PostZone)
	ct.NodeMapDefault = make(map[string]Peer)
	ct.ZoneConfigDefault = make(map[string]ZoneConfig)
	ct.setZoneDefaultDetails(DefaultZoneName)
	ct.streamSocket = fmt.Sprintf("%s:%d", *streamService, streamPort)
	ct.streamPass = *streamPasswd

	return ct
}

func main() {
	ct := initApp()
	client := NewRedisClient(ct.streamSocket, ct.streamPass)
	defer client.Close()

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Unable to connect to the redis instance at %s: %v", ct.streamSocket, err)
	}

	// respond to initial health check from agents initializing
	readyCheckResponder(ctx, client)

	// Handle all messages for zones other than the default zone
	// TODO: assign each zone it's own channel for better multi-tenancy
	go ct.MessageHandling(ctx)

	// Initialize ipam for the default zone
	ctxDefault := context.Background()
	ctIpamDefault, err := ipam.NewIPAM(ctx, DefaultIpamSaveFile, ipPrefixDefault)
	if err != nil {
		log.Warnf("failed to acquire an ipam address %v\n", err)
	}

	pubDefault := NewPubsub(NewRedisClient(ct.streamSocket, ct.streamPass))
	subDefault := NewPubsub(NewRedisClient(ct.streamSocket, ct.streamPass))

	log.Debugf("Listening on channel: %s", zoneChannelDefault)

	// channel for async messages from the zone subscription for the default zone
	msgChanDefault := make(chan string)

	go func() {
		subDefault.subscribe(ctx, zoneChannelDefault, msgChanDefault)
		for {
			msg := <-msgChanDefault
			msgEvent := handleMsg(msg)
			switch msgEvent.Event {
			case registerNodeRequest:
				log.Debugf("Register node msg received on channel [ %s ]\n", zoneChannelDefault)
				log.Debugf("Received registration request: %+v\n", msgEvent.Peer)
				if msgEvent.Peer.PublicKey != "" {
					nodeEvent := Peer{}
					var ip string
					// If this was a static address request
					if msgEvent.Peer.NodeAddress != "" {
						if err := ipam.ValidateIp(msgEvent.Peer.NodeAddress); err == nil {
							ip, err = ctIpamDefault.RequestSpecificIP(ctxDefault, msgEvent.Peer.NodeAddress, ipPrefixDefault)
							if err != nil {
								log.Errorf("failed to assign the requested address, assigning an address from the pool %v\n", err)
								ip, err = ctIpamDefault.RequestIP(ctxDefault, ipPrefixDefault)
								if err != nil {
									log.Errorf("failed to acquire an IPAM assigned address %v\n", err)
								}
							}
						}
					} else {
						ip, err = ctIpamDefault.RequestIP(ctxDefault, ipPrefixDefault)
						if err != nil {
							log.Errorf("failed to acquire an IPAM assigned address %v\n", err)
						}
					}
					// allocate a child prefix if requested
					var childPrefix string
					if msgEvent.Peer.ChildPrefix != "" {
						childPrefix, err = ctIpamDefault.RequestChildPrefix(ctxDefault, msgEvent.Peer.ChildPrefix)
						if err != nil {
							log.Errorf("%v\n", err)
						}
					}
					// save the ipam to persistent storage
					ctIpamDefault.IpamSave(ctxDefault)
					// construct the new node
					nodeEvent = msgEvent.newNode(ip, childPrefix)
					log.Debugf("node allocated: %+v\n", nodeEvent)
					// delete the old k/v pair if one exists and replace it with the new registration data
					if _, ok := ct.NodeMapDefault[msgEvent.Peer.PublicKey]; ok {
						delete(ct.NodeMapDefault, msgEvent.Peer.PublicKey)
					}
					ct.NodeMapDefault[msgEvent.Peer.PublicKey] = nodeEvent
					// append all peers into the updated peer list to be published
					var peerList []Peer
					for pubKey, nodeElements := range ct.NodeMapDefault {
						log.Printf("NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s] NodeAddress [%s] Zone [%s] ChildPrefix [%s]\n",
							pubKey, nodeElements.EndpointIP, nodeElements.AllowedIPs, nodeElements.NodeAddress, nodeElements.Zone, nodeElements.ChildPrefix)
						// append the new node to the updated peer listing
						peerList = append(peerList, nodeElements)
					}
					pubDefault.publishPeers(ctx, zoneChannelDefault, peerList)
				}
			}
		}
	}()

	// Start the http router, this is blocking
	ginSocket := fmt.Sprintf("localhost:%s", restPort)
	ct.Router.Run(ginSocket)

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
func (ct *Controltower) setZoneDefaultDetails(zone string) {
	zoneConfDefault := ZoneConfig{
		Name:        zone,
		Description: "Default Zone",
		IpCidr:      ipPrefixDefault,
	}
	ct.ZoneConfigDefault[zone] = zoneConfDefault
}
