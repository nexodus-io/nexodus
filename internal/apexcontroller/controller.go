package apexcontroller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	_ "github.com/redhat-et/apex/internal/docs"
	log "github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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
	Router      *gin.Engine
	db          *gorm.DB
	dbHost      string
	dbPass      string
	ipam        apiv1connect.IpamServiceClient
	wg          sync.WaitGroup
	server      *http.Server
	defaultZone uuid.UUID
}

func NewController(ctx context.Context, keyCloakAddr string, dbHost string, dbPass string, ipamAddress string) (*Controller, error) {
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
	err := backoff.Retry(connectDb, backoff.NewExponentialBackOff())
	if err != nil {
		return nil, err
	}

	// Migrate the schema
	if err := db.AutoMigrate(&User{}); err != nil {
		return nil, err
	}
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

	log.Debug("Waiting for Keycloak")
	connectKeycloak := func() error {
		kcHealthURL := fmt.Sprintf("http://%s:8080/auth/health/ready", keyCloakAddr)
		log.Debugf("Ready url %s", kcHealthURL)
		res, err := http.Get(kcHealthURL)
		if err != nil {
			return err
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			return err
		}

		if _, ok := response["status"]; !ok {
			return fmt.Errorf("no status")
		}

		if response["status"] != "UP" {
			return fmt.Errorf("not ready")
		}
		return nil
	}

	err = backoff.Retry(connectKeycloak, backoff.NewExponentialBackOff())
	if err != nil {
		return nil, err
	}

	jwksURL := fmt.Sprintf("http://%s:8080/auth/realms/controller/protocol/openid-connect/certs", keyCloakAddr)
	log.Debugf("cert url %s", jwksURL)
	auth, err := NewKeyCloakAuth(jwksURL)
	if err != nil {
		return nil, err
	}

	ct := &Controller{
		Router: gin.Default(),
		db:     db,
		dbHost: dbHost,
		dbPass: dbPass,
		ipam:   ipam,
		wg:     sync.WaitGroup{},
		server: nil,
	}

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	ct.Router.Use(cors.New(corsConfig))
	ct.Router.Use()

	private := ct.Router.Group("/api")
	ct.Router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	private.Use(auth.AuthFunc())
	private.Use(ct.UserMiddleware)
	private.GET("/zones", ct.handleListZones)
	private.POST("/zones", ct.handlePostZones)
	private.GET("/zones/:zone", ct.handleGetZones)
	private.GET("/zones/:zone/peers", ct.handleListZonePeers)
	private.POST("/zones/:zone/peers", ct.handlePostZonePeers)
	private.GET("/zones/:zone/peers/:id", ct.handleGetZonePeers)
	private.GET("/devices", ct.handleListDevices)
	private.GET("/devices/:id", ct.handleGetDevices)
	private.POST("/devices", ct.handlePostDevices)
	private.GET("/peers", ct.handleListPeers)
	private.GET("/peers/:id", ct.handleGetPeers)
	private.GET("/users/:id", ct.handleGetUser)
	private.GET("/users", ct.handleListUsers)
	private.PATCH("/users/:id", ct.handlePatchUser)

	ct.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
	// Shutdown REST API
	if err := ct.server.Shutdown(ctx); err != nil {
		return err
	}
	// Wait for everything to wind down
	ct.wg.Wait()

	return nil
}

// setZoneDetails set default zone attributes
func (ct *Controller) CreateDefaultZone() error {
	var zone Zone
	log.Debug("Checking that Default Zone exists")
	res := ct.db.Where("name = ?", DefaultZoneName).First(&zone)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		log.Debug("Creating Default Zone")
		zone, err := ct.NewZone(DefaultZoneName, "Default Zone", ipPrefixDefault, false)
		if err != nil {
			return err
		}
		log.Debugf("Default Zone UUID is: %s", zone.ID)
		ct.defaultZone = zone.ID
		return nil
	}
	log.Debug("Default Zone Already Created")
	log.Debugf("Default Zone UUID is: %s", zone.ID)
	ct.defaultZone = zone.ID
	return nil
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
