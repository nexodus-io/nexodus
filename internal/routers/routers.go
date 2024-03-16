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

	r.Use(NoCacheMiddleware)
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

	deviceGroup := r.Group("/device", loggerMiddleware)
	{
		deviceGroup.POST("/login/start", o.DeviceFlow.DeviceStart)
		deviceGroup.GET("/certs", o.Api.Certs)
	}
	webGroup := r.Group("/web", loggerMiddleware)
	{
		webGroup.Use(o.BrowserFlow.OriginVerifier())
		webGroup.Use(ginsession.New(
			session.SetCookieName(handlers.SESSION_ID_COOKIE_NAME),
			session.SetStore(o.SessionStore)))
		webGroup.GET("/login/start", o.BrowserFlow.LoginStart)
		webGroup.GET("/login/end", o.BrowserFlow.LoginEnd)
		webGroup.GET("/user_info", o.BrowserFlow.UserInfo)
		webGroup.GET("/claims", o.BrowserFlow.Claims)
		webGroup.GET("/logout", o.BrowserFlow.Logout)
		// web.GET("/check_auth", o.BrowserFlow.CheckAuth)
		webGroup.POST("/refresh", o.BrowserFlow.Refresh)
	}
	apiGroup := r.Group("/api", loggerMiddleware)
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

		apiGroup.Use(validateJWT)

		// Feature Flags
		apiGroup.GET("fflags", api.ListFeatureFlags)
		apiGroup.GET("fflags/:name", api.GetFeatureFlag)

		// Users
		apiGroup.GET("/users", api.ListUsers)
		apiGroup.GET("/users/:id", api.GetUser)
		apiGroup.DELETE("/users/:id", api.DeleteUser)
		apiGroup.DELETE("/users/:id/organizations/:organization", api.DeleteUserFromOrganization)

		// Organizations
		apiGroup.GET("/organizations", api.ListOrganizations)
		apiGroup.POST("/organizations", api.CreateOrganization)
		apiGroup.GET("/organizations/:id", api.GetOrganizations)
		apiGroup.DELETE("/organizations/:id", api.DeleteOrganization)

		apiGroup.GET("/organizations/:id/users", api.ListOrganizationUsers)
		apiGroup.GET("/organizations/:id/users/:uid", api.GetOrganizationUser)
		apiGroup.DELETE("/organizations/:id/users/:uid", api.DeleteOrganizationUser)

		// Invitations
		apiGroup.GET("/invitations", api.ListInvitations)
		apiGroup.GET("/invitations/:id", api.GetInvitation)
		apiGroup.POST("/invitations", api.CreateInvitation)
		apiGroup.POST("/invitations/:id/accept", api.AcceptInvitation)
		apiGroup.DELETE("/invitations/:id", api.DeleteInvitation)

		apiGroup.GET("/vpcs", api.ListVPCs)
		apiGroup.GET("/vpcs/:id", api.GetVPC)
		apiGroup.PATCH("/vpcs/:id", api.UpdateVPC)
		apiGroup.POST("/vpcs", api.CreateVPC)
		apiGroup.DELETE("/vpcs/:id", api.DeleteVPC)

		// Registration Tokens
		apiGroup.GET("/reg-keys", api.ListRegKeys)
		apiGroup.GET("/reg-keys/:id", api.GetRegKey)
		apiGroup.POST("/reg-keys", api.CreateRegKey)
		apiGroup.PATCH("/reg-keys/:id", api.UpdateRegKey)
		apiGroup.DELETE("/reg-keys/:id", api.DeleteRegKey)

		// Devices
		apiGroup.GET("/devices", api.ListDevices)
		apiGroup.GET("/devices/:id", api.GetDevice)
		apiGroup.PATCH("/devices/:id", api.UpdateDevice)
		apiGroup.POST("/devices", api.CreateDevice)
		apiGroup.DELETE("/devices/:id", api.DeleteDevice)

		// Device Metadata
		apiGroup.GET("/devices/:id/metadata", api.ListDeviceMetadata)
		apiGroup.GET("/devices/:id/metadata/:key", api.GetDeviceMetadataKey)
		apiGroup.PUT("/devices/:id/metadata/:key", api.UpdateDeviceMetadataKey)
		apiGroup.DELETE("/devices/:id/metadata/:key", api.DeleteDeviceMetadataKey)
		apiGroup.DELETE("/devices/:id/metadata", api.DeleteDeviceMetadata)

		// Sites
		apiGroup.GET("/sites", api.ListSites)
		apiGroup.GET("/sites/:id", api.GetSite)
		apiGroup.PATCH("/sites/:id", api.UpdateSite)
		apiGroup.POST("/sites", api.CreateSite)
		apiGroup.DELETE("/sites/:id", api.DeleteSite)

		// Security Groups
		apiGroup.GET("/security-groups", api.ListSecurityGroups)
		apiGroup.GET("/security-groups/:id", api.GetSecurityGroup)
		apiGroup.POST("/security-groups", api.CreateSecurityGroup)
		apiGroup.PATCH("/security-groups/:id", api.UpdateSecurityGroup)
		apiGroup.DELETE("/security-groups/:id", api.DeleteSecurityGroup)

		// Status
		apiGroup.POST("/status", api.CreateStatus)
		apiGroup.GET("/status", api.GetStatus)

		// List / Watch Event API used by nexd
		apiGroup.POST("/vpcs/:id/events", api.WatchEvents)
		apiGroup.GET("/vpcs/:id/devices", api.ListDevicesInVPC)
		apiGroup.GET("/vpcs/:id/sites", api.ListSitesInVPC)
		apiGroup.GET("/vpcs/:id/metadata", api.ListMetadataInVPC)
		apiGroup.GET("/vpcs/:id/security-groups", api.ListSecurityGroupsInVPC)

		apiGroup.POST("/ca/sign", api.SignCSR)
	}

	privateGroup := r.Group("/private")
	{
		privateGroup.GET("/gc", o.Api.GarbageCollect, loggerMiddleware)
		privateGroup.GET("/ready", o.Api.Ready)
		privateGroup.GET("/live", o.Api.Live)
	}

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
