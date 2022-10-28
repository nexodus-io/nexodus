package apexcontroller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/bufbuild/connect-go"
	"github.com/cenkalti/backoff/v4"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apiv1 "github.com/metal-stack/go-ipam/api/v1"
	"github.com/metal-stack/go-ipam/api/v1/apiv1connect"
	"github.com/redhat-et/apex/internal/messages"
	"github.com/redhat-et/apex/internal/streamer"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	ipPrefixDefault = "10.200.1.0/20"
	DefaultZoneName = "default"
	restPort        = "8080"
)

// Controller specific data
type Controller struct {
	Router       *gin.Engine
	Client       *streamer.Streamer
	db           *gorm.DB
	dbHost       string
	dbPass       string
	ipam         apiv1connect.IpamServiceClient
	streamerIp   string
	streamerPort int
	streamerPass string
	wg           sync.WaitGroup
	server       *http.Server
	msgChannels  []chan streamer.ReceivedMessage
	readyChan    chan streamer.ReceivedMessage
}

func NewController(ctx context.Context, streamerIp string, streamerPort int, streamerPass string, dbHost string, dbPass string, ipamAddress string) (*Controller, error) {
	st := streamer.NewStreamer(ctx, streamerIp, streamerPort, streamerPass)
	log.Debug("Waiting for Streamer")
	checkRedis := func() error {
		if !st.IsReady() {
			return fmt.Errorf("streamer is not ready")
		}
		return nil
	}
	err := backoff.Retry(checkRedis, backoff.NewExponentialBackOff())
	if err != nil {
		return nil, fmt.Errorf("Streamer is not ready at %s", st.GetUrl())
	}
	log.Debugf("Streamer is ready and reachable")

	dsn := fmt.Sprintf("host=%s user=controller password=%s dbname=controller port=5432 sslmode=disable", dbHost, dbPass)
	var db *gorm.DB
	connectDb := func() error {
		var err error
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			return err
		}
		return nil
	}
	log.Debug("Waiting for Postgres")
	err = backoff.Retry(connectDb, backoff.NewExponentialBackOff())
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

	ipam := apiv1connect.NewIpamServiceClient(
		http.DefaultClient,
		ipamAddress,
		connect.WithGRPC(),
	)

	jwksURL := "http://keycloak:8080/realms/controller/protocol/openid-connect/certs"
	var auth *KeyCloakAuth
	connectAuth := func() error {
		var err error
		auth, err = NewKeyCloakAuth(jwksURL)
		if err != nil {
			return err
		}
		return nil
	}
	log.Debug("Waiting for Keycloak Auth")
	err = backoff.Retry(connectAuth, backoff.NewExponentialBackOff())
	if err != nil {
		return nil, err
	}

	ct := &Controller{
		Router:       gin.Default(),
		Client:       st,
		db:           db,
		dbHost:       dbHost,
		dbPass:       dbPass,
		ipam:         ipam,
		streamerIp:   streamerIp,
		streamerPort: streamerPort,
		streamerPass: streamerPass,
		wg:           sync.WaitGroup{},
		server:       nil,
		msgChannels:  make([]chan streamer.ReceivedMessage, 0),
		readyChan:    make(chan streamer.ReceivedMessage),
	}

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	ct.Router.Use(cors.New(corsConfig))
	ct.Router.Use()

	public := ct.Router.Group("/health")
	public.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	private := ct.Router.Group("/")
	private.Use(auth.AuthFunc())
	private.GET("/peers", ct.handleListPeers)    // http://localhost:8080/peers
	private.GET("/peers/:id", ct.handleGetPeers) // http://localhost:8080/peers/:id
	private.GET("/zones", ct.handleListZones)    // http://localhost:8080/zones
	private.GET("/zones/:id", ct.handleGetZones) // http://localhost:8080/zones/:id
	private.POST("/zones", ct.handlePostZones)
	private.GET("/devices", ct.handleListDevices)    // http://localhost:8080/devices
	private.GET("/devices/:id", ct.handleGetDevices) // http://localhost:8080/devices/:id
	private.POST("/devices", ct.handlePostDevices)

	createDefaultZone := func() error {
		if err := ct.CreateDefaultZone(); err != nil {
			log.Errorf("Error creating default zone: %s", err)
			return err
		}
		return nil
	}
	log.Debug("Waiting for Default Zone to create successfully")
	err = backoff.Retry(createDefaultZone, backoff.NewExponentialBackOff())
	if err != nil {
		return nil, err
	}
	return ct, nil
}

func (ct *Controller) Run() {
	ctx := context.Background()

	// respond to initial health check from agents initializing
	readyCheckResponder(ctx, ct.Client, ct.readyChan, &ct.wg)

	// Handle all messages for zones other than the default zone
	// TODO: assign each zone it's own channel for better multi-tenancy
	go ct.MessageHandling(ctx)

	pubDefault := streamer.NewStreamer(ctx, ct.streamerIp, ct.streamerPort, ct.streamerPass)
	subDefault := streamer.NewStreamer(ctx, ct.streamerIp, ct.streamerPort, ct.streamerPass)

	log.Debugf("Listening on channel: %s", messages.ZoneChannelDefault)

	// channel for async messages from the zone subscription for the default zone
	msgChanDefault := make(chan streamer.ReceivedMessage)
	ct.msgChannels = append(ct.msgChannels, msgChanDefault)
	ct.wg.Add(1)
	go func() {
		subDefault.SubscribeAndReceive(messages.ZoneChannelDefault, msgChanDefault)
		defer subDefault.CloseSubscription(messages.ZoneChannelDefault)
		for {
			msg, ok := <-msgChanDefault
			if !ok {
				log.Infof("Shutting down default message channel")
				ct.wg.Done()
				return
			}
			msgEvent := messages.HandleMessage(msg.Payload)
			switch msgEvent.Event {
			case messages.RegisterNodeRequest:
				log.Debugf("Register node msg received on channel [ %s ]\n", messages.ZoneChannelDefault)
				log.Debugf("Received registration request: %+v\n", msgEvent.Peer)
				if msgEvent.Peer.PublicKey != "" {
					zone, err := ct.zoneExists(ctx, msgEvent.Peer.ZoneID)
					if err != nil {
						log.Error(err)
						if err = publishErrorMessage(pubDefault,
							msgEvent.Peer.ZoneID, messages.Error, messages.ChannelNotRegistered, err.Error()); err != nil {
							log.Errorf("unable to publish error message %s", err)
						}
					} else {
						peers, err := ct.AddPeer(ctx, msgEvent, zone)
						if err == nil {
							// publishPeers the latest peer list
							if err := publishPeers(pubDefault, messages.ZoneChannelDefault, peers); err != nil {
								log.Errorf("unable to publish peer list: %s", err)
							}
						} else {
							log.Errorf("peer was not added: %v", err)
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

func (ct *Controller) Shutdown(ctx context.Context) error {
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
func (ct *Controller) CreateDefaultZone() error {
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

func (ct *Controller) zoneExists(ctx context.Context, zoneId string) (*Zone, error) {
	var zone Zone
	res := ct.db.Preload("Peers").First(&zone, "id = ?", zoneId)
	// todo, the needs to go over an err channel to the agent
	if res.Error != nil && errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return &zone, fmt.Errorf("requested zone [ %s ] was not found.", zoneId)
	}
	return &zone, nil

}
func (ct *Controller) AddPeer(ctx context.Context, msgEvent messages.Message, zone *Zone) ([]messages.Peer, error) {
	tx := ct.db.Begin()

	ipamPrefix := zone.IpCidr
	var hubZone bool
	var hubRouter bool
	// determine if the node joining is a hub-router or in a hub-zone
	if msgEvent.Peer.HubRouter && zone.HubZone {
		hubRouter = true
		// If the zone already has a hub-router, reject the join of a new node trying to be a zone-router
		var hubRouterAlreadyExists string
		for _, p := range zone.Peers {
			// If the node joining is a re-join from the zone-router allow it
			if p.HubRouter && p.DeviceID != msgEvent.Peer.PublicKey {
				hubRouterAlreadyExists = p.EndpointIP
				return nil, fmt.Errorf("hub router already exists on the endpoint %s", hubRouterAlreadyExists)
			}
		}
	}
	if zone.HubZone {
		hubZone = true
	}

	var key Device
	res := ct.db.First(&key, "id = ?", msgEvent.Peer.PublicKey)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			log.Debug("Public key not found. Adding a new one")
			key = Device{
				ID: msgEvent.Peer.PublicKey,
			}
			tx.Create(&key)
		} else {
			return nil, res.Error
		}
	}
	var found bool
	var peer *Peer
	for _, p := range zone.Peers {
		if p.DeviceID == msgEvent.Peer.PublicKey {
			found = true
			peer = p
			break
		}
	}
	if found {
		peer.EndpointIP = msgEvent.Peer.EndpointIP

		if msgEvent.Peer.NodeAddress != peer.NodeAddress {
			var ip string
			if msgEvent.Peer.NodeAddress != "" {
				var err error
				ip, err = ct.assignSpecificNodeAddress(ctx, ipamPrefix, msgEvent.Peer.NodeAddress)
				if err != nil {
					return nil, err
				}
			} else {
				if peer.NodeAddress == "" {
					return nil, fmt.Errorf("peer does not have a node address assigned in the peer table and did not request a specifc address")
				}
				ip = peer.NodeAddress
			}
			peer.NodeAddress = ip
			peer.AllowedIPs = ip
		}

		if msgEvent.Peer.ChildPrefix != peer.ChildPrefix {
			if err := ct.assignChildPrefix(ctx, msgEvent.Peer.ChildPrefix); err != nil {
				return nil, err
			}
		}
	} else {
		log.Debugf("Public key not in the zone %s. Creating a new peer", zone.ID)
		var ip string
		// If this was a static address request
		// TODO: handle a user requesting an IP not in the IPAM prefix
		if msgEvent.Peer.NodeAddress != "" {
			var err error
			ip, err = ct.assignSpecificNodeAddress(ctx, ipamPrefix, msgEvent.Peer.NodeAddress)
			if err != nil {
				return nil, err
			}
		} else {
			var err error
			ip, err = ct.assignFromPool(ctx, ipamPrefix)
			if err != nil {
				return nil, err
			}
		}
		// allocate a child prefix if requested
		if msgEvent.Peer.ChildPrefix != "" {
			if err := ct.assignChildPrefix(ctx, msgEvent.Peer.ChildPrefix); err != nil {
				return nil, err
			}
		}
		peer = &Peer{
			ID:          uuid.New().String(),
			DeviceID:    key.ID,
			ZoneID:      zone.ID,
			EndpointIP:  msgEvent.Peer.EndpointIP,
			AllowedIPs:  ip,
			NodeAddress: ip,
			ChildPrefix: msgEvent.Peer.ChildPrefix,
			ZonePrefix:  ipamPrefix,
			HubZone:     hubZone,
			HubRouter:   hubRouter,
		}
		tx.Create(&peer)
		zone.Peers = append(zone.Peers, peer)
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
	tx.Save(&peer)
	if err := tx.Commit(); err.Error != nil {
		tx.Rollback()
		return nil, err.Error
	}
	return peerList, nil
}

func (ct *Controller) MessageHandling(ctx context.Context) {

	pub := streamer.NewStreamer(ctx, ct.streamerIp, ct.streamerPort, ct.streamerPass)
	sub := streamer.NewStreamer(ctx, ct.streamerIp, ct.streamerPort, ct.streamerPass)

	// channel for async messages from the zone subscription
	controllerChan := make(chan streamer.ReceivedMessage)

	go func() {
		sub.SubscribeAndReceive(messages.ZoneChannelController, controllerChan)
		defer sub.CloseSubscription(messages.ZoneChannelController)
		log.Debugf("Listening on channel: %s", messages.ZoneChannelController)

		for {
			msg := <-controllerChan
			msgEvent := messages.HandleMessage(msg.Payload)
			switch msgEvent.Event {
			// TODO implement error channels
			case messages.RegisterNodeRequest:
				log.Debugf("Register node msg received on channel [ %s ]\n", messages.ZoneChannelController)
				log.Debugf("Received registration request: %+v\n", msgEvent.Peer)
				if msgEvent.Peer.PublicKey != "" {
					zone, err := ct.zoneExists(ctx, msgEvent.Peer.ZoneID)
					if err != nil {
						log.Error(err)
						if err = publishErrorMessage(
							pub, msgEvent.Peer.ZoneID, messages.Error, messages.ChannelNotRegistered, err.Error()); err != nil {
							log.Errorf("failed to publish error message %s", err)
						}
					} else {
						peers, err := ct.AddPeer(ctx, msgEvent, zone)
						if err == nil {
							// publishPeers the latest peer list
							if err := publishPeers(pub, msgEvent.Peer.ZoneID, peers); err != nil {
								log.Errorf("failed to publish peer updates: %s", err)
							}
						} else {

							log.Errorf("peer was not added: %v", err)
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

func (ct *Controller) assignSpecificNodeAddress(ctx context.Context, ipamPrefix string, nodeAddress string) (string, error) {
	if err := validateIP(nodeAddress); err != nil {
		return "", fmt.Errorf("Address %s is not valid, assigning from zone pool: %s", nodeAddress, ipamPrefix)
	}
	res, err := ct.ipam.AcquireIP(ctx, connect.NewRequest(&apiv1.AcquireIPRequest{
		PrefixCidr: ipamPrefix,
		Ip:         &nodeAddress,
	}))
	if err != nil {
		log.Errorf("failed to assign the requested address %s, assigning an address from the pool: %v\n", nodeAddress, err)
		return ct.assignFromPool(ctx, ipamPrefix)
	}
	return res.Msg.Ip.Ip, nil
}

func (ct *Controller) assignFromPool(ctx context.Context, ipamPrefix string) (string, error) {
	res, err := ct.ipam.AcquireIP(ctx, connect.NewRequest(&apiv1.AcquireIPRequest{
		PrefixCidr: ipamPrefix,
	}))
	if err != nil {
		log.Errorf("failed to acquire an IPAM assigned address %v", err)
		return "", fmt.Errorf("failed to acquire an IPAM assigned address %v\n", err)
	}
	return res.Msg.Ip.Ip, nil
}

func (ct *Controller) assignChildPrefix(ctx context.Context, cidr string) error {
	cidr, err := cleanCidr(cidr)
	if err != nil {
		log.Errorf("invalid child prefix requested: %v", err)
		return fmt.Errorf("invalid child prefix requested: %v", err)
	}
	_, err = ct.ipam.CreatePrefix(ctx, connect.NewRequest(&apiv1.CreatePrefixRequest{Cidr: cidr}))
	return err
}
