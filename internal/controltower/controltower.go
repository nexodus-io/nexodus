package controltower

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	goipam "github.com/metal-stack/go-ipam"
	"github.com/redhat-et/jaywalking/internal/messages"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	ipPrefixDefault = "10.200.1.0/20"
	DefaultZoneName = "default"
	restPort        = "8080"
)

// Control tower specific data
type ControlTower struct {
	Router       *gin.Engine
	Client       *redis.Client
	db           *gorm.DB
	dbHost       string
	dbPass       string
	ipam         map[string]goipam.Ipamer
	streamSocket string
	streamPass   string
	wg           sync.WaitGroup
	server       *http.Server
	msgChannels  []chan string
	readyChan    chan string
}

func NewControlTower(ctx context.Context, streamService string, streamPort int, streamPasswd string, dbHost string, dbPass string) (*ControlTower, error) {
	streamSocket := fmt.Sprintf("%s:%d", streamService, streamPort)
	client := NewRedisClient(streamSocket, streamPasswd)
	dsn := fmt.Sprintf("host=%s user=controltower password=%s dbname=controltower port=5432 sslmode=disable", dbHost, dbPass)

	var db *gorm.DB
	connectDb := func() error {
		var err error
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			return err
		}
		return nil
	}
	err := backoff.Retry(connectDb, backoff.NewExponentialBackOff())
	if err != nil {
		return nil, err
	}

	// Migrate the schema
	if err := db.AutoMigrate(&Zone{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Peer{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Device{}); err != nil {
		return nil, err
	}

	_, err = client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Unable to connect to the redis instance at %s: %v", streamSocket, err)
	}

	ct := &ControlTower{
		Router:       gin.Default(),
		Client:       client,
		db:           db,
		dbHost:       dbHost,
		dbPass:       dbPass,
		ipam:         make(map[string]goipam.Ipamer),
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
	ct.Router.GET("/peers", ct.handleListPeers)    // http://localhost:8080/peers
	ct.Router.GET("/peers/:id", ct.handleGetPeers) // http://localhost:8080/peers/:id
	ct.Router.GET("/zones", ct.handleListZones)    // http://localhost:8080/zones
	ct.Router.GET("/zones/:id", ct.handleGetZones) // http://localhost:8080/zones/:id
	ct.Router.POST("/zones", ct.handlePostZones)
	ct.Router.GET("/devices", ct.handleListDevices)    // http://localhost:8080/devices
	ct.Router.GET("/devices/:id", ct.handleGetDevices) // http://localhost:8080/devices/:id

	if err := ct.CreateDefaultZone(); err != nil {
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
					zone, err := ct.zoneExists(ctx, msgEvent.Peer.ZoneID)
					if err != nil {
						log.Error(err)
						if _, err = pubDefault.publishErrorMessage(
							ctx, msgEvent.Peer.ZoneID, messages.Error, messages.ChannelNotRegistered, err.Error()); err != nil{
							log.Errorf("Unable to publish error message %s", err)
						}
					} else {
						peers, err := ct.AddPeer(ctx, msgEvent, zone)
						if err == nil {
							// publishPeers the latest peer list
							if _, err := pubDefault.publishPeers(ctx, messages.ZoneChannelDefault, peers); err != nil {
								log.Errorf("unable to publish peer list: %s", err)
							}
						} else {
							log.Errorf("Peer was not added: %v", err)
						}
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
func (ct *ControlTower) CreateDefaultZone() error {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	var zone Zone
	log.Debug("Checking that Default Zone exists")
	res := ct.db.First(&zone, "id = ?", id.String())
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		log.Debug("Creating Default Zone")
		_, err := ct.NewZone(id.String(), DefaultZoneName, "Default Zone", ipPrefixDefault, false)
		if err != nil {
			return err
		}
		return nil
	}
	log.Debug("Default Zone Already Created")
	return res.Error
}

type MsgTypes struct {
	ID    string
	Event string
	Zone  string
	Peer  Peer
}

func (ct *ControlTower) zoneExists (ctx context.Context, zoneId string) (*Zone, error) {
	var zone Zone
	res := ct.db.Preload("Peers").First(&zone, "id = ?", zoneId)
	// todo, the needs to go over an err channel to the agent
	if res.Error != nil && errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return &zone, fmt.Errorf("requested zone [ %s ] was not found.", zoneId)
	}
	return &zone, nil

}
 func (ct *ControlTower) AddPeer(ctx context.Context, msgEvent messages.Message, zone *Zone) ([]messages.Peer, error) {
	tx := ct.db.Begin()

	ipamPrefix := zone.IpCidr
	var hubZone bool
	var hubRouter bool
	// determine if the node joining is a hub-router or in a hub-zone
	if msgEvent.Peer.HubRouter && zone.HubZone {
		hubRouter = true
	}
	if zone.HubZone {
		hubZone = true
	}

	var key Device
	res := ct.db.First(&key, "id = ?", msgEvent.Peer.PublicKey)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			log.Debug("Public Key Not Found. Adding A New One")
			key = Device{
				ID: msgEvent.Peer.PublicKey,
			}
			tx.Create(&key)
		} else {
			return nil, res.Error
		}
	}

	var found bool
	var peer Peer
	for _, p := range zone.Peers {
		if p.DeviceID == msgEvent.Peer.PublicKey {
			found = true
			peer = p
			break
		}
	}
	if !found {
		log.Debugf("PublicKey Not In Zone %s. Creating a New Peer", zone.ID)
		var ip *goipam.IP
		// If this was a static address request
		// TODO: handle a user requesting an IP not in the IPAM prefix
		if msgEvent.Peer.NodeAddress != "" {
			if err := validateIP(msgEvent.Peer.NodeAddress); err == nil {
				ip, err = ct.ipam[zone.ID].AcquireSpecificIP(ctx, msgEvent.Peer.NodeAddress, ipamPrefix)
				if err != nil {
					log.Errorf("failed to assign the requested address %s, assigning an address from the pool %v\n", msgEvent.Peer.NodeAddress, err)
					ip, err = ct.ipam[zone.ID].AcquireIP(ctx, ipamPrefix)
					if err != nil {
						return nil, fmt.Errorf("failed to acquire an IPAM assigned address %v\n", err)
					}
				}
			}
		} else {
			var err error
			ip, err = ct.ipam[zone.ID].AcquireIP(ctx, ipamPrefix)
			if err != nil {
				return nil, fmt.Errorf("failed to acquire an IPAM assigned address %v\n", err)
			}
		}
		// allocate a child prefix if requested
		if msgEvent.Peer.ChildPrefix != "" {
			cidr, err := cleanCidr(msgEvent.Peer.ChildPrefix)
			if err != nil {
				return nil, fmt.Errorf("invalid child prefix requested: %v", err)
			}
			_, err = ct.ipam[zone.ID].NewPrefix(ctx, cidr)
			if err != nil {
				log.Errorf("%v\n", err)
			}
		}
		peer = Peer{
			ID:          uuid.New().String(),
			DeviceID:    key.ID,
			ZoneID:      zone.ID,
			EndpointIP:  msgEvent.Peer.EndpointIP,
			AllowedIPs:  ip.IP.String(),
			NodeAddress: ip.IP.String(),
			ChildPrefix: msgEvent.Peer.ChildPrefix,
			ZonePrefix:  ipamPrefix,
			HubZone:     hubZone,
			HubRouter:   hubRouter,
		}
		tx.Create(&peer)
	}
	zone.Peers = append(zone.Peers, peer)
	tx.Save(&zone)
	if err := tx.Commit(); err.Error != nil {
		tx.Rollback()
		return nil, err.Error
	}

	var peerList []messages.Peer
	for _, p := range zone.Peers {
		peerList = append(peerList, messages.Peer{
			PublicKey:   p.DeviceID,
			ZoneID:      p.ZoneID,
			EndpointIP:  p.EndpointIP,
			AllowedIPs:  p.AllowedIPs,
			NodeAddress: p.NodeAddress,
			ChildPrefix: p.ChildPrefix,
			HubRouter:   p.HubRouter,
			HubZone:     p.HubZone,
			ZonePrefix:  p.ZonePrefix,
		})
	}
	return peerList, nil
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
				log.Debugf("Received registration request: %+v\n", msgEvent.Peer)
				if msgEvent.Peer.PublicKey != "" {
					zone, err := ct.zoneExists(ctx, msgEvent.Peer.ZoneID)
					if err != nil {
						log.Error(err)
						if _, err = pub.publishErrorMessage(
							ctx, msgEvent.Peer.ZoneID, messages.Error, messages.ChannelNotRegistered, err.Error()); err != nil{
							log.Errorf("failed to publish error message %s", err)
						}
					} else {
						peers, err := ct.AddPeer(ctx, msgEvent, zone)
						if err == nil {
							// publishPeers the latest peer list
							if _, err := pub.publishPeers(ctx, msgEvent.Peer.ZoneID, peers); err != nil {
								log.Errorf("failed to publish peer updates: %s", err)
							}
						} else {
							
							log.Errorf("Peer was not added: %v", err)
						}
					}

				}
			}
		}
	}()
}

// cleanCidr ensures a valid IP4/IP6 address is provided and return a proper
// network prefix if the network address if the network address was not precise.
// example: if a user provides 192.168.1.1/24 we will infer 192.168.1.0/24.
func cleanCidr(uncheckedCidr string) (string, error) {
	_, validCidr, err := net.ParseCIDR(uncheckedCidr)
	if err != nil {
		return "", err
	}
	return validCidr.String(), nil
}

// ValidateIP ensures a valid IP4/IP6 address is provided
func validateIP(ip string) error {
	if ip := net.ParseIP(ip); ip != nil {
		return nil
	}
	return fmt.Errorf("%s is not a valid v4 or v6 IP", ip)
}
