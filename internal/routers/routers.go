package routers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	_ "github.com/redhat-et/apex/internal/docs"
	"github.com/redhat-et/apex/internal/handlers"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

const name = "github.com/redhat-et/apex/internal/routers"

func NewAPIRouter(
	ctx context.Context,
	logger *zap.SugaredLogger,
	api *handlers.API,
	clientIdWeb string,
	clientIdCli string,
	oidcURL string) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	p := ginprometheus.NewPrometheus("apiserver")
	p.ReqCntURLLabelMappingFn = func(c *gin.Context) string {
		url := c.Request.URL.Path
		for _, p := range c.Params {
			if p.Key == "id" {
				url = strings.Replace(url, p.Value, ":id", 1)
				break
			}
			// If zone cardinality is too big we'll replace here too
		}
		return url
	}
	p.Use(r)

	r.Use(ginzap.Ginzap(logger.Desugar(), time.RFC3339, true))
	r.Use(ginzap.RecoveryWithZap(logger.Desugar(), true))
	r.Use(otelgin.Middleware(name))

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
		private.Use(ValidateJWT(logger, verifier, clientIdWeb, clientIdCli))
		private.Use(api.CreateUserIfNotExists())
		// Zones
		private.GET("/zones", api.ListZones)
		private.POST("/zones", api.CreateZone)
		private.GET("/zones/:zone", api.GetZones)
		private.DELETE("/zones/:zone", api.DeleteZone)
		private.GET("/zones/:zone/peers", api.ListPeersInZone)
		private.POST("/zones/:zone/peers", api.CreatePeerInZone)
		private.GET("/zones/:zone/peers/:id", api.GetPeerInZone)
		// Devices
		private.GET("/devices", api.ListDevices)
		private.GET("/devices/:id", api.GetDevice)
		private.POST("/devices", api.CreateDevice)
		private.DELETE("/devices/:id", api.DeleteDevice)
		// Peers
		private.GET("/peers", api.ListPeers)
		private.GET("/peers/:id", api.GetPeers)
		private.DELETE("/peers/:id", api.DeletePeer)
		// Users
		private.GET("/users/:id", api.GetUser)
		private.GET("/users", api.ListUsers)
		private.PATCH("/users/:id", api.PatchUser)
		private.DELETE("/users/:id", api.DeleteUser)
	}

	r.GET("/api/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	return r, nil
}
