package controltower

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/redhat-et/jaywalking/internal/controltower/ipam"
	"github.com/redhat-et/jaywalking/internal/messages"
	log "github.com/sirupsen/logrus"
)

const (
	ipPrefixDefault = "10.200.1.0/20"
	DefaultZoneName = "default"
	restPort        = "8080"
)

// Control tower specific data
type ControlTower struct {
	Router       *gin.Engine
	Zones        map[uuid.UUID]*Zone
	PubKeys      map[string]*PubKey
	Peers        map[uuid.UUID]*Peer
	Client       *redis.Client
	streamSocket string
	streamPass   string
	wg           sync.WaitGroup
	server       *http.Server
	msgChannels  []chan string
	readyChan    chan string
}

func NewControlTower(ctx context.Context, streamService string, streamPort int, streamPasswd string) (*ControlTower, error) {
	streamSocket := fmt.Sprintf("%s:%d", streamService, streamPort)
	client := NewRedisClient(streamSocket, streamPasswd)

	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Unable to connect to the redis instance at %s: %v", streamSocket, err)
	}

	ct := &ControlTower{
		Router:       gin.Default(),
		Zones:        make(map[uuid.UUID]*Zone),
		PubKeys:      make(map[string]*PubKey),
		Peers:        make(map[uuid.UUID]*Peer),
		Client:       client,
		streamSocket: streamSocket,
		streamPass:   streamPasswd,
		wg:           sync.WaitGroup{},
		server:       nil,
		msgChannels:  make([]chan string, 0),
		readyChan:    make(chan string),
	}

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	ct.Router.Use(cors.New(corsConfig))
	ct.Router.GET("/peers", ct.GetPeers)        // http://localhost:8080/peers
	ct.Router.GET("/peers/:id", ct.GetPeer)     // http://localhost:8080/peers/:id
	ct.Router.GET("/zones", ct.GetZones)        // http://localhost:8080/zones
	ct.Router.GET("/zones/:id", ct.GetZone)     // http://localhost:8080/zones/:id
	ct.Router.GET("/pubkeys", ct.GetPubKeys)    // http://localhost:8080/pubkeys
	ct.Router.GET("/pubkeys/:id", ct.GetPubKey) // http://localhost:8080/oubkeys/:id
	ct.Router.POST("/zones", ct.PostZone)
	if err := ct.setZoneDefaultDetails(); err != nil {
		return nil, err
	}
	return ct, nil
}

func (ct *ControlTower) Run() {
	ctx := context.Background()

	// respond to initial health check from agents initializing
	readyCheckResponder(ctx, ct.Client, ct.readyChan, &ct.wg)

	// Handle all messages for zones other than the default zone
	// TODO: assign each zone it's own channel for better multi-tenancy
	go ct.MessageHandling(ctx)

	pubDefault := NewPubsub(NewRedisClient(ct.streamSocket, ct.streamPass))
	subDefault := NewPubsub(NewRedisClient(ct.streamSocket, ct.streamPass))

	log.Debugf("Listening on channel: %s", messages.ZoneChannelDefault)

	// channel for async messages from the zone subscription for the default zone
	msgChanDefault := make(chan string)
	ct.msgChannels = append(ct.msgChannels, msgChanDefault)
	ct.wg.Add(1)
	go func() {
		subDefault.subscribe(ctx, messages.ZoneChannelDefault, msgChanDefault)
		for {
			msg, ok := <-msgChanDefault
			if !ok {
				log.Infof("Shutting down default message channel")
				ct.wg.Done()
				return
			}
			msgEvent := messages.HandleMessage(msg)
			switch msgEvent.Event {
			case messages.RegisterNodeRequest:
				log.Debugf("Register node msg received on channel [ %s ]\n", messages.ZoneChannelDefault)
				log.Debugf("Received registration request: %+v\n", msgEvent.Peer)
				if msgEvent.Peer.PublicKey != "" {
					err := ct.AddPeer(ctx, msgEvent)
					if err == nil {
						var peerList []Peer
						for _, zone := range ct.Zones {
							if zone.Name == msgEvent.Peer.Zone {
								for id := range zone.Peers {
									nodeElements, ok := ct.Peers[id]
									if !ok {
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
							if _, err := pubDefault.publishPeers(ctx, messages.ZoneChannelDefault, peerList); err != nil {
								log.Errorf("unable to publish peer list: %s", err)
							}
						}
					} else {
						log.Errorf("Peer was not added: %v", err)
						// TODO: return an error to the agent on a message chan
					}
				}
			}
		}
	}()

	ct.server = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%s", restPort),
		Handler: ct.Router,
	}

	ct.wg.Add(1)
	go func() {
		defer ct.wg.Done()
		if err := ct.server.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			log.Infof("REST API shutting down: %s\n", err)
		}
	}()
}

func (ct *ControlTower) Shutdown(ctx context.Context) error {
	// Shutdown Ready Checker
	close(ct.readyChan)

	// Shutdown REST API
	if err := ct.server.Shutdown(ctx); err != nil {
		return err
	}

	// Shutdown Message Handlers
	for _, c := range ct.msgChannels {
		close(c)
	}

	// Wait for everything to wind down
	ct.wg.Wait()

	return nil
}

// setZoneDetails set default zone attributes
func (ct *ControlTower) setZoneDefaultDetails() error {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	z, err := NewZone(id, DefaultZoneName, "Default Zone", ipPrefixDefault)
	if err != nil {
		return err
	}
	ct.Zones[id] = z
	return nil
}

type MsgTypes struct {
	ID    string
	Event string
	Zone  string
	Peer  Peer
}

func (ct *ControlTower) AddPeer(ctx context.Context, msgEvent messages.Message) error {
	var ipamPrefix string
	var err error
	var z *Zone
	for _, zone := range ct.Zones {
		if msgEvent.Peer.Zone == zone.Name {
			ipamPrefix = zone.IpCidr
			z = zone
		}
	}
	// todo, the needs to go over an err channal to the agent
	if z == nil {
		return fmt.Errorf("requested zone [ %s ] was not found, has it been created yet?", msgEvent.Peer.Zone)
	}

	// Have we seen this pubKey before?
	if pk, ok := ct.PubKeys[msgEvent.Peer.PublicKey]; ok {
		for id := range pk.Peers {
			if ct.Peers[id].Zone == z.Name {
				// TODO: Do other things need updating here too?
				ct.Peers[id].EndpointIP = msgEvent.Peer.EndpointIP
				return nil
			}
		}

	} else {
		pk := NewPubKey(msgEvent.Peer.PublicKey)
		ct.PubKeys[msgEvent.Peer.PublicKey] = pk
	}
	var ip string
	// If this was a static address request
	// TODO: handle a user requesting an IP not in the IPAM prefix
	if msgEvent.Peer.NodeAddress != "" {
		if err := ipam.ValidateIp(msgEvent.Peer.NodeAddress); err == nil {
			ip, err = z.ZoneIpam.RequestSpecificIP(ctx, msgEvent.Peer.NodeAddress, ipamPrefix)
			if err != nil {
				log.Errorf("failed to assign the requested address %s, assigning an address from the pool %v\n", msgEvent.Peer.NodeAddress, err)
				ip, err = z.ZoneIpam.RequestIP(ctx, ipamPrefix)
				if err != nil {
					return fmt.Errorf("failed to acquire an IPAM assigned address %v\n", err)
				}
			}
		}
	} else {
		ip, err = z.ZoneIpam.RequestIP(ctx, ipamPrefix)
		if err != nil {
			return fmt.Errorf("failed to acquire an IPAM assigned address %v\n", err)
		}
	}
	// allocate a child prefix if requested
	var childPrefix string
	if msgEvent.Peer.ChildPrefix != "" {
		childPrefix, err = z.ZoneIpam.RequestChildPrefix(ctx, msgEvent.Peer.ChildPrefix)
		if err != nil {
			log.Errorf("%v\n", err)
		}
	}
	// save the ipam to persistent storage
	if err := z.ZoneIpam.IpamSave(ctx); err != nil {
		return err
	}

	// construct the new peer
	peer := NewPeer(
		msgEvent.Peer.PublicKey,
		msgEvent.Peer.EndpointIP,
		ip, // This will be a slice, NodeAddress will hold the /32
		msgEvent.Peer.Zone,
		ip,
		childPrefix,
	)

	log.Debugf("node allocated: %+v\n", peer)
	ct.Peers[peer.ID] = &peer
	ct.PubKeys[peer.PublicKey].Peers[peer.ID] = struct{}{}
	z.Peers[peer.ID] = struct{}{}
	log.Infof("Zone has %d peers", len(z.Peers))
	log.Infof("Zone %+v", ct.Zones[z.ID])
	return nil
}

func (ct *ControlTower) MessageHandling(ctx context.Context) {

	pub := NewPubsub(NewRedisClient(ct.streamSocket, ct.streamPass))
	sub := NewPubsub(NewRedisClient(ct.streamSocket, ct.streamPass))

	// channel for async messages from the zone subscription
	controllerChan := make(chan string)

	go func() {
		sub.subscribe(ctx, messages.ZoneChannelController, controllerChan)
		log.Debugf("Listening on channel: %s", messages.ZoneChannelController)

		for {
			msg := <-controllerChan
			msgEvent := messages.HandleMessage(msg)
			switch msgEvent.Event {
			// TODO implement error chans
			case messages.RegisterNodeRequest:
				log.Debugf("Register node msg received on channel [ %s ]\n", messages.ZoneChannelController)
				log.Debugf("Recieved registration request: %+v\n", msgEvent.Peer)
				if msgEvent.Peer.PublicKey != "" {
					err := ct.AddPeer(ctx, msgEvent)
					// append all peers into the updated peer list to be published
					if err == nil {
						var peerList []Peer
						for _, zone := range ct.Zones {
							if zone.Name == msgEvent.Peer.Zone {
								for id := range zone.Peers {
									nodeElements, ok := ct.Peers[id]
									if !ok {
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
							if _, err := pub.publishPeers(ctx, messages.ZoneChannelController, peerList); err != nil {
								log.Errorf("unable to publish peer updates: %s", err)
							}
						}
					} else {
						log.Errorf("Peer was not added: %v", err)
						// TODO: return an error to the agent on a message chan
					}
				}
			}
		}
	}()
}

type ZoneRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IpCidr      string `json:"cidr"`
}

// PostZone creates a new zone via a REST call
func (ct *ControlTower) PostZone(c *gin.Context) {
	var request ZoneRequest
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if request.IpCidr == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the zone request did not contain a required CIDR prefix"})
		return
	}
	if request.Name == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "the zone request did not contain a required name"})
		return
	}

	// Create the zone
	newZone, err := NewZone(uuid.New(), request.Name, request.Description, request.IpCidr)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "unable to create zone"})
		return
	}

	// TODO: From this point on Zone should get cleaned up if we fail
	for _, zone := range ct.Zones {
		if zone.Name == newZone.Name {
			failMsg := fmt.Sprintf("%s zone already exists", newZone.Name)
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": failMsg})
			return
		}
	}
	log.Debugf("New zone request [ %s ] and ipam [ %s ] request", newZone.Name, newZone.IpCidr)
	ct.Zones[newZone.ID] = newZone
	c.IndentedJSON(http.StatusCreated, newZone)
}

// GetZones responds with the list of all peers as JSON.
func (ct *ControlTower) GetZones(c *gin.Context) {
	allZones := make([]*Zone, 0)
	for _, z := range ct.Zones {
		allZones = append(allZones, z)
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(allZones)))
	c.JSON(http.StatusOK, allZones)
}

// GetZone responds with a single zone
func (ct *ControlTower) GetZone(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zone id is not a valid UUID"})
		return
	}
	if value, ok := ct.Zones[k]; ok {
		c.JSON(http.StatusOK, value)
	} else {
		c.Status(http.StatusNotFound)
	}
}

// GetPeers responds with the list of all peers as JSON. TODO: Currently default zone only
func (ct *ControlTower) GetPeers(c *gin.Context) {
	allPeers := make([]*Peer, 0)
	for _, p := range ct.Peers {
		allPeers = append(allPeers, p)
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(allPeers)))
	c.JSON(http.StatusOK, allPeers)
}

// GetPeer locates a peer
func (ct *ControlTower) GetPeer(c *gin.Context) {
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "peer id is not a valid UUID"})
		return
	}
	if value, ok := ct.Peers[k]; ok {
		c.JSON(http.StatusOK, value)
	} else {
		c.Status(http.StatusNotFound)
	}
}

// GetPubKeys responds with the list of all peers as JSON. TODO: Currently default zone only
func (ct *ControlTower) GetPubKeys(c *gin.Context) {
	allPubKeys := make([]*PubKey, 0)
	for _, p := range ct.PubKeys {
		allPubKeys = append(allPubKeys, p)
	}
	// For pagination
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")
	c.Header("X-Total-Count", strconv.Itoa(len(allPubKeys)))
	c.JSON(http.StatusOK, allPubKeys)
}

// GetPubKey locates a peer
func (ct *ControlTower) GetPubKey(c *gin.Context) {
	k := c.Param("id")
	if k == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pubkey id is not valid"})
		return
	}
	if value, ok := ct.PubKeys[k]; ok {
		c.JSON(http.StatusOK, value)
	} else {
		c.Status(http.StatusNotFound)
	}
}
