package routers

import (
	"context"
	"crypto/tls"
	"github.com/coreos/go-oidc/v3/oidc"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	_ "github.com/nexodus-io/nexodus/internal/docs"
	"github.com/nexodus-io/nexodus/internal/handlers"
	agent "github.com/nexodus-io/nexodus/pkg/oidcagent"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

const name = "github.com/nexodus-io/nexodus/internal/routers"

func NewAPIRouter(
	ctx context.Context,
	logger *zap.SugaredLogger,
	api *handlers.API,
	clientIdWeb string,
	clientIdCli string,
	oidcURL string,
	oidcBackchannel string,
	insecureTLS bool,
	browserFlow *agent.OidcAgent,
	deviceFlow *agent.OidcAgent,
) (*gin.Engine, error) {

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	loggerMiddleware := ginzap.GinzapWithConfig(logger.Desugar(), &ginzap.Config{TimeFormat: time.RFC3339, UTC: true, TraceID: true})
	r.Use(otelgin.Middleware(name))
	r.Use(ginzap.RecoveryWithZap(logger.Desugar(), true))

	r.Use(browserFlow.CorsMiddleware())
	r.Use(browserFlow.CookieSessionMiddleware())
	agent.AddCodeFlowRoutes(r.Group("/browser", loggerMiddleware), browserFlow)
	agent.AddDeviceFlowRoutes(r.Group("/device", loggerMiddleware), deviceFlow)

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

	if insecureTLS {
		transport := &http.Transport{
			// #nosec -- G402: TLS InsecureSkipVerify set true.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: transport}
		ctx = oidc.ClientContext(ctx, client)
	}

	if oidcBackchannel != "" {
		ctx = oidc.InsecureIssuerURLContext(ctx,
			oidcURL,
		)
		oidcURL = oidcBackchannel
	}
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

	private := r.Group("/api", loggerMiddleware)
	{
		private.Use(ValidateJWT(logger, verifier, clientIdWeb, clientIdCli))
		private.Use(api.CreateUserIfNotExists())
		// Zones
		private.GET("/organizations", api.ListOrganizations)
		private.POST("/organizations", api.CreateOrganization)
		private.GET("/organizations/:organization", api.GetOrganizations)
		private.DELETE("/organizations/:organization", api.DeleteOrganization)
		private.GET("/organizations/:organization/devices", api.ListDevicesInOrganization)
		private.GET("/organizations/:organization/devices/:id", api.GetDeviceInOrganization)
		// Devices
		private.GET("/devices", api.ListDevices)
		private.GET("/devices/:id", api.GetDevice)
		private.PATCH("/devices/:id", api.UpdateDevice)
		private.POST("/devices", api.CreateDevice)
		private.DELETE("/devices/:id", api.DeleteDevice)
		// Users
		private.GET("/users/:id", api.GetUser)
		private.GET("/users", api.ListUsers)
		// private.PATCH("/users/:id", api.PatchUser)
		private.DELETE("/users/:id", api.DeleteUser)
		// Feature Flags
		private.GET("fflags", api.ListFeatureFlags)
		private.GET("fflags/:name", api.GetFeatureFlag)
	}

	r.GET("/api/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler), loggerMiddleware)

	// Don't log the health/readiness checks.
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "UP",
		})
	})
	r.GET("/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "UP",
		})
	})

	return r, nil
}
