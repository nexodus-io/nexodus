package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
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
	ID          uuid.UUID `json:"id"`
	PublicKey   string    `json:"public-key"`
	EndpointIP  string    `json:"endpoint-ip"`
	AllowedIPs  string    `json:"allowed-ips"`
	Zone        string    `json:"zone"`
	NodeAddress string    `json:"node-address"`
	ChildPrefix string    `json:"child-prefix"`
}

type ZoneConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IpCidr      string `json:"cidr"`
}

type MsgEvent struct {
	Event string
	Peer  Peer
}

type Zone struct {
	ID          uuid.UUID
	Peers       map[uuid.UUID]struct{}
	Name        string
	Description string
	IpCidr      string
	ZoneIpam    ipam.AirliftIpam
}

func NewZone(id uuid.UUID, name string, description string, cidr string) (*Zone, error) {
	zoneIpamSaveFile := fmt.Sprintf("%s.json", id.String())
	// TODO: until we save control tower state between restarts, the ipam save file will be out of sync
	// new zones will delete the stale IPAM file on creation.
	// currently this will delete and overwrite an existing zone and ipam objects.
	if fileExists(zoneIpamSaveFile) {
		log.Warnf("ipam persistent storage file [ %s ] already exists on the control tower, deleting it", zoneIpamSaveFile)
		if err := deleteFile(zoneIpamSaveFile); err != nil {
			return nil, fmt.Errorf("unable to delete the ipam persistent storage file on the control tower [ %s ]: %v", zoneIpamSaveFile, err)
		}
	}
	ipam, err := ipam.NewIPAM(context.Background(), zoneIpamSaveFile, cidr)
	if err != nil {
		return nil, err
	}
	if err := ipam.IpamSave(context.Background()); err != nil {
		log.Errorf("failed to save the ipam persistent storage file %v", err)
		return nil, err
	}
	return &Zone{
		ID:          id,
		Peers:       make(map[uuid.UUID]struct{}),
		Name:        name,
		Description: description,
		IpCidr:      cidr,
		ZoneIpam:    *ipam,
	}, nil
}

func (z *Zone) MarshalJSON() ([]byte, error) {
	peers := make([]uuid.UUID, 0)
	for k := range z.Peers {
		peers = append(peers, k)
	}
	return json.Marshal(
		struct {
			ID          uuid.UUID   `json:"id"`
			Peers       []uuid.UUID `json:"peers"`
			Name        string      `json:"name"`
			Description string      `json:"description"`
			IpCidr      string      `json:"cidr"`
		}{
			ID:          z.ID,
			Peers:       peers,
			Name:        z.Name,
			Description: z.Description,
			IpCidr:      z.IpCidr,
		})
}

type PeerMap struct {
	cache   map[uuid.UUID]*Peer
	pubKeys map[string]uuid.UUID
}

func NewPeerMap() *PeerMap {
	return &PeerMap{
		cache:   make(map[uuid.UUID]*Peer),
		pubKeys: make(map[string]uuid.UUID),
	}
}

func (m *PeerMap) InsertOrUpdate(p Peer) uuid.UUID {
	if id, ok := m.pubKeys[p.PublicKey]; ok {
		m.cache[id] = &p
		return id
	} else {
		m.cache[p.ID] = &p
		m.pubKeys[p.PublicKey] = p.ID
		return p.ID
	}
}

func (m *PeerMap) List() []*Peer {
	res := make([]*Peer, 0)
	for _, v := range m.cache {
		res = append(res, v)
	}
	return res
}

func (m *PeerMap) ListByPubKey(key string) []*Peer {
	if v, ok := m.pubKeys[key]; ok {
		return []*Peer{m.cache[v]}
	}
	return nil
}

func (m *PeerMap) Get(id uuid.UUID) (*Peer, error) {
	if peer, ok := m.cache[id]; ok {
		return peer, nil
	}
	return nil, fmt.Errorf("peer not found")
}

// Control tower specific data
type Controltower struct {
	Router       *gin.Engine
	Zones        map[uuid.UUID]*Zone
	Peers        *PeerMap
	streamSocket string
	streamPass   string
}

func initApp() *Controltower {
	ct := new(Controltower)
	ct.Router = gin.Default()
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	ct.Router.Use(cors.New(corsConfig))
	ct.Router.GET("/peers", ct.GetPeers)    // http://localhost:8080/peers TODO: only functioning for zone:default atm
	ct.Router.GET("/peers/:id", ct.GetPeer) // http://localhost:8080/peers/id
	ct.Router.GET("/zones", ct.GetZones)    // http://localhost:8080/zones
	ct.Router.GET("/zones/:id", ct.GetZone)
	ct.Router.POST("/zones", ct.PostZone)
	ct.Peers = NewPeerMap()
	ct.Zones = make(map[uuid.UUID]*Zone)
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
					err := ct.AddPeer(ctx, msgEvent)
					if err == nil {
						var peerList []Peer
						for _, zone := range ct.Zones {
							if zone.Name == msgEvent.Peer.Zone {
								for id := range zone.Peers {
									nodeElements, err := ct.Peers.Get(id)
									if err != nil {
										log.Errorf("unable to find peer with id %s", id.String())
										continue
									}
									log.Printf("NodeState - PublicKey: [%s] EndpointIP [%s] AllowedIPs [%s] NodeAddress [%s] Zone [%s] ChildPrefix [%s]\n",
										nodeElements.PublicKey, nodeElements.EndpointIP, nodeElements.AllowedIPs, nodeElements.NodeAddress, nodeElements.Zone, nodeElements.ChildPrefix)
									// append the new node to the updated peer listing
									peerList = append(peerList, *nodeElements)
								}
							}
							// publishPeers the latest peer list
							pubDefault.publishPeers(ctx, zoneChannelDefault, peerList)
						}
					} else {
						log.Errorf("Peer was not added: %v", err)
						// TODO: return an error to the agent on a message chan
					}
				}
			}
		}
	}()

	// Start the http router, this is blocking
	ginSocket := fmt.Sprintf("0.0.0.0:%s", restPort)
	ct.Router.Run(ginSocket)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	<-ch
}

func (msgEvent *MsgEvent) newNode(ipamIP, childPrefix string) Peer {
	peer := Peer{
		ID:          uuid.New(),
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
		log.Debugf("handleMsg unmarshall error: %v\n", err)
		return peer
	}
	return peer
}

// setZoneDetails set default zone attributes
func (ct *Controltower) setZoneDefaultDetails(zone string) error {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	z, err := NewZone(id, zone, "Default Zone", ipPrefixDefault)
	if err != nil {
		return err
	}
	ct.Zones[id] = z
	return nil
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
