package routers

import (
	"context"
	"crypto/tls"
	"github.com/go-session/session/v3"
	"github.com/nexodus-io/nexodus/internal/docs"
	"github.com/nexodus-io/nexodus/pkg/ginsession"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap/zapcore"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	_ "github.com/nexodus-io/nexodus/internal/docs"
	"github.com/nexodus-io/nexodus/internal/handlers"
	agent "github.com/nexodus-io/nexodus/pkg/oidcagent"
	"github.com/open-policy-agent/opa/storage"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

const name = "github.com/nexodus-io/nexodus/internal/routers"

type APIRouterOptions struct {
	Logger          *zap.SugaredLogger
	Api             *handlers.API
	ClientIdWeb     string
	ClientIdCli     string
	OidcURL         string
	OidcBackchannel string
	InsecureTLS     bool
	BrowserFlow     *agent.OidcAgent
	DeviceFlow      *agent.OidcAgent
	Store           storage.Store
	SessionStore    session.ManagerStore
}

func NewAPIRouter(ctx context.Context, o APIRouterOptions) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	loggerMiddleware := ginzap.GinzapWithConfig(o.Logger.Desugar(), &ginzap.Config{
		TimeFormat: time.RFC3339,
		UTC:        true,
		Context: func(c *gin.Context) []zapcore.Field {
			return []zapcore.Field{
				zap.String("traceID", trace.SpanFromContext(c.Request.Context()).SpanContext().TraceID().String()),
			}
		},
	})

	r.Use(otelgin.Middleware(name, otelgin.WithPropagators(
		propagation.TraceContext{},
	)))
	r.Use(ginzap.RecoveryWithZap(o.Logger.Desugar(), true))

	newPrometheus().Use(r)

	u, err := url.Parse(o.Api.URL)
	if err != nil {
		return nil, err
	}
	docs.SwaggerInfo.Schemes = []string{u.Scheme}
	docs.SwaggerInfo.Host = u.Host

	r.GET("/openapi/*any", ginSwagger.WrapHandler(swaggerFiles.Handler), loggerMiddleware)

	device := r.Group("/device", loggerMiddleware)
	{
		device.POST("/login/start", o.DeviceFlow.DeviceStart)
		device.GET("/certs", o.Api.Certs)
	}
	web := r.Group("/web", loggerMiddleware)
	{
		web.Use(o.BrowserFlow.OriginVerifier())
		web.Use(ginsession.New(
			session.SetCookieName(handlers.SESSION_ID_COOKIE_NAME),
			session.SetStore(o.SessionStore)))
		web.POST("/login/start", o.BrowserFlow.LoginStart)
		web.POST("/login/end", o.BrowserFlow.LoginEnd)
		web.GET("/user_info", o.BrowserFlow.UserInfo)
		web.GET("/claims", o.BrowserFlow.Claims)
		web.POST("/logout", o.BrowserFlow.Logout)
		// web.GET("/check_auth", o.BrowserFlow.CheckAuth)
		web.POST("/refresh", o.BrowserFlow.Refresh)
	}
	private := r.Group("/api", loggerMiddleware)
	{
		api := o.Api
		nexodusJWKS, err := api.JSONWebKeySet()
		if err != nil {
			return nil, err
		}

		validateJWT, err := newValidateJWT(ctx, o, string(nexodusJWKS))
		if err != nil {
			return nil, err
		}

		private.Use(validateJWT)

		// Feature Flags
		private.GET("fflags", api.ListFeatureFlags)
		private.GET("fflags/:name", api.GetFeatureFlag)

		// Users
		private.GET("/users", api.ListUsers)
		private.GET("/users/:id", api.GetUser)
		private.DELETE("/users/:id", api.DeleteUser)
		private.DELETE("/users/:id/organizations/:organization", api.DeleteUserFromOrganization)

		// Organizations
		private.GET("/organizations", api.ListOrganizations)
		private.POST("/organizations", api.CreateOrganization)
		private.GET("/organizations/:id", api.GetOrganizations)
		private.DELETE("/organizations/:id", api.DeleteOrganization)

		// Invitations
		private.GET("/invitations", api.ListInvitations)
		private.GET("/invitations/:id", api.GetInvitation)
		private.POST("/invitations", api.CreateInvitation)
		private.POST("/invitations/:id/accept", api.AcceptInvitation)
		private.DELETE("/invitations/:id", api.DeleteInvitation)

		private.GET("/vpcs", api.ListVPCs)
		private.GET("/vpcs/:id", api.GetVPC)
		private.PATCH("/vpcs/:id", api.UpdateVPC)
		private.POST("/vpcs", api.CreateVPC)
		private.DELETE("/vpcs/:id", api.DeleteVPC)

		// Registration Tokens
		private.GET("/reg-keys", api.ListRegKeys)
		private.GET("/reg-keys/:id", api.GetRegKey)
		private.POST("/reg-keys", api.CreateRegKey)
		private.PATCH("/reg-keys/:id", api.UpdateRegKey)
		private.DELETE("/reg-keys/:id", api.DeleteRegKey)

		// Devices
		private.GET("/devices", api.ListDevices)
		private.GET("/devices/:id", api.GetDevice)
		private.PATCH("/devices/:id", api.UpdateDevice)
		private.POST("/devices", api.CreateDevice)
		private.DELETE("/devices/:id", api.DeleteDevice)

		// Device Metadata
		private.GET("/devices/:id/metadata", api.ListDeviceMetadata)
		private.GET("/devices/:id/metadata/:key", api.GetDeviceMetadataKey)
		private.PUT("/devices/:id/metadata/:key", api.UpdateDeviceMetadataKey)
		private.DELETE("/devices/:id/metadata/:key", api.DeleteDeviceMetadataKey)
		private.DELETE("/devices/:id/metadata", api.DeleteDeviceMetadata)

		// Sites
		private.GET("/sites", api.ListSites)
		private.GET("/sites/:id", api.GetSite)
		private.PATCH("/sites/:id", api.UpdateSite)
		private.POST("/sites", api.CreateSite)
		private.DELETE("/sites/:id", api.DeleteSite)

		// Security Groups
		private.GET("/security-groups", api.ListSecurityGroups)
		private.GET("/security-groups/:id", api.GetSecurityGroup)
		private.POST("/security-groups", api.CreateSecurityGroup)
		private.PATCH("/security-groups/:id", api.UpdateSecurityGroup)
		private.DELETE("/security-groups/:id", api.DeleteSecurityGroup)

		// List / Watch Event API used by nexd
		private.POST("/vpcs/:id/events", api.WatchEvents)
		private.GET("/vpcs/:id/devices", api.ListDevicesInVPC)
		private.GET("/vpcs/:id/sites", api.ListSitesInVPC)
		private.GET("/vpcs/:id/metadata", api.ListMetadataInVPC)
		private.GET("/vpcs/:id/security-groups", api.ListSecurityGroupsInVPC)

	}

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

func newValidateJWT(ctx context.Context, o APIRouterOptions, nexodusJWKS string) (func(*gin.Context), error) {
	if o.InsecureTLS {
		transport := &http.Transport{
			// #nosec -- G402: TLS InsecureSkipVerify set true.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: transport}
		ctx = oidc.ClientContext(ctx, client)
	}

	oidcURL := o.OidcURL
	if o.OidcBackchannel != "" {
		ctx = oidc.InsecureIssuerURLContext(ctx, o.OidcURL)
		oidcURL = o.OidcBackchannel
	}
	provider, err := oidc.NewProvider(ctx, oidcURL)
	if err != nil {
		return nil, err
	}

	var claims struct {
		JWKSUri string `json:"jwks_uri"`
	}
	err = provider.Claims(&claims)
	if err != nil {
		return nil, err
	}

	return ValidateJWT(ctx, o, claims.JWKSUri, nexodusJWKS)
}

func newPrometheus() *ginprometheus.Prometheus {
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
	return p
}
