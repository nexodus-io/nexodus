package routers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cenkalti/backoff"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/redhat-et/apex/internal/docs"
	"github.com/redhat-et/apex/internal/handlers"
	log "github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func NewRouter(api *handlers.API, keycloakAddress string) (*gin.Engine, error) {
	r := gin.Default()

	log.Debug("Waiting for Keycloak")
	connectKeycloak := func() error {
		res, err := http.Get(fmt.Sprintf("http://%s:8080/auth/health/ready", keycloakAddress))
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

	err := backoff.Retry(connectKeycloak, backoff.NewExponentialBackOff())
	if err != nil {
		return nil, err
	}

	jwksURL := fmt.Sprintf("http://%s:8080/auth/realms/controller/protocol/openid-connect/certs", keycloakAddress)

	auth, err := NewKeyCloakAuth(jwksURL)
	if err != nil {
		return nil, err
	}

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	r.Use(cors.New(corsConfig))

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	private := r.Group("/api")
	{
		private.Use(auth.AuthFunc())
		private.Use(api.CreateUserIfNotExists())
		// Zones
		private.GET("/zones", api.ListZones)
		private.POST("/zones", api.CreateZone)
		private.GET("/zones/:zone", api.GetZones)
		private.GET("/zones/:zone/peers", api.ListPeersInZone)
		private.POST("/zones/:zone/peers", api.CreatePeerInZone)
		private.GET("/zones/:zone/peers/:id", api.GetPeerInZone)
		// Devices
		private.GET("/devices", api.ListDevices)
		private.GET("/devices/:id", api.GetDevice)
		private.POST("/devices", api.CreateDevice)
		// Peers
		private.GET("/peers", api.ListPeers)
		private.GET("/peers/:id", api.GetPeers)
		// Users
		private.GET("/users/:id", api.GetUser)
		private.GET("/users", api.ListUsers)
		private.PATCH("/users/:id", api.PatchUser)
	}

	r.GET("/api/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return r, nil
}
