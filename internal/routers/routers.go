package routers

import (
	"context"
	"crypto/tls"
	"github.com/go-session/session/v3"
	"github.com/nexodus-io/nexodus/pkg/ginsession"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/otel/propagation"
	"net/http"
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

	loggerMiddleware := ginzap.GinzapWithConfig(o.Logger.Desugar(), &ginzap.Config{TimeFormat: time.RFC3339, UTC: true, TraceID: true})

	r.Use(otelgin.Middleware(name, otelgin.WithPropagators(
		propagation.TraceContext{},
	)))
	r.Use(ginzap.RecoveryWithZap(o.Logger.Desugar(), true))

	newPrometheus().Use(r)

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
		private.Use(api.CreateUserIfNotExists())
		// Organizations
		private.GET("/organizations", api.ListOrganizations)
		private.POST("/organizations", api.CreateOrganization)
		private.GET("/organizations/:organization", api.GetOrganizations)
		private.DELETE("/organizations/:organization", api.DeleteOrganization)
		private.GET("/organizations/:organization/devices", api.ListDevicesInOrganization)
		private.GET("/organizations/:organization/devices/:id", api.GetDeviceInOrganization)
		private.GET("/organizations/:organization/users", api.ListUsersInOrganization)
		// Invitations
		private.POST("/invitations", api.CreateInvitation)
		private.GET("/invitations", api.ListInvitations)
		private.GET("/invitations/:invitation", api.GetInvitation)
		private.POST("/invitations/:invitation/accept", api.AcceptInvitation)
		private.DELETE("/invitations/:invitation", api.DeleteInvitation)
		// Registration Tokens
		private.GET("/registration-tokens", api.ListRegistrationTokens)
		private.GET("/registration-tokens/:token-id", api.GetRegistrationToken)
		private.POST("/registration-tokens", api.CreateRegistrationToken)
		private.DELETE("/registration-tokens/:token-id", api.DeleteRegistrationToken)
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
		private.DELETE("/devices/:id/metadata", api.DeleteDeviceMetadata)
		private.DELETE("/devices/:id/metadata/:key", api.DeleteDeviceMetadataKey)
		private.GET("/organizations/:organization/metadata", api.ListOrganizationMetadata)

		// Users
		private.GET("/users/:id", api.GetUser)
		private.GET("/users", api.ListUsers)
		// private.PATCH("/users/:id", api.PatchUser)
		private.DELETE("/users/:id", api.DeleteUser)
		private.DELETE("/users/:id/organizations/:organization", api.DeleteUserFromOrganization)
		// Security Groups
		private.POST("/organizations/:organization/security_groups", api.CreateSecurityGroup)
		private.GET("/organizations/:organization/security_groups", api.ListSecurityGroups)
		private.DELETE("/organizations/:organization/security_groups/:id", api.DeleteSecurityGroup)
		private.GET("/organizations/:organization/security_group/:id", api.GetSecurityGroup)
		private.PATCH("/organizations/:organization/security_groups/:id", api.UpdateSecurityGroup)

		// Event APIs
		private.POST("/organizations/:organization/events", api.WatchEvents)

		// Feature Flags
		private.GET("fflags", api.ListFeatureFlags)
		private.GET("fflags/:name", api.GetFeatureFlag)
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
