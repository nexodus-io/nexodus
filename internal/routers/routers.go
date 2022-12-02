package routers

import (
	"context"
	"net/http"

	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	_ "github.com/redhat-et/apex/internal/docs"
	"github.com/redhat-et/apex/internal/handlers"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func NewAPIRouter(
	ctx context.Context,
	api *handlers.API,
	clientIdWeb string,
	clientIdCli string,
	oidcURL string) (*gin.Engine, error) {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	provider, err := oidc.NewProvider(ctx, oidcURL)
	if err != nil {
		return nil, err
	}
	config := &oidc.Config{
		// Client ID checks are skipped since we perform these later
		// in the ValidateJWT function
		SkipClientIDCheck: true,
	}
	verifier := provider.Verifier(config)

	private := r.Group("/")
	{
		private.Use(ValidateJWT(verifier, clientIdWeb, clientIdCli))
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
