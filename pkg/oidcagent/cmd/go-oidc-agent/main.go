package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	agent "github.com/redhat-et/go-oidc-agent"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const (
	OidcProviderArg     = "oidc-provider"
	OidcClientIDArg     = "oidc-client-id"
	OidcClientSecretArg = "oidc-client-secret"
	RedirectURLArg      = "redirect-url"
	ScopesArg           = "scopes"
	OriginsArg          = "origins"
	DomainArg           = "domain"
	BackendArg          = "backend"
	CookieKeyArg        = "cookie-key"
	FlowArg             = "flow"
)

func main() {
	app := &cli.App{
		Name: "go-oidc-agent",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Enable Debug Logging",
				Value:   false,
				EnvVars: []string{"DEBUG"},
			},
			&cli.StringFlag{
				Name:  FlowArg,
				Usage: "OAuth2 Flow",
				Value: "authorization",
				Action: func(ctx *cli.Context, s string) error {
					if s != "authorization" && s != "device" {
						return fmt.Errorf("flag 'flow' value should be one of 'authorization' or 'device'")
					}
					return nil
				},
				EnvVars: []string{"OIDC_FLOW"},
			},
			&cli.StringFlag{
				Name:    OidcProviderArg,
				Usage:   "OIDC Provider URL",
				Value:   "https://accounts.google.com",
				EnvVars: []string{"OIDC_PROVIDER"},
			},
			&cli.StringFlag{
				Name:    OidcClientIDArg,
				Usage:   "OIDC Client ID",
				Value:   "my-app-id",
				EnvVars: []string{"OIDC_CLIENT_ID"},
			},
			&cli.StringFlag{
				Name:    OidcClientSecretArg,
				Usage:   "OIDC Client Secret",
				Value:   "secret",
				EnvVars: []string{"OIDC_CLIENT_SECRET"},
			},
			&cli.StringFlag{
				Name:    RedirectURLArg,
				Usage:   "Redirect URL. This is the URL of the SPA.",
				Value:   "https://example.com",
				EnvVars: []string{"REDIRECT_URL"},
			},
			&cli.StringSliceFlag{
				Name:    ScopesArg,
				Usage:   "Additional OAUTH2 scopes",
				Value:   &cli.StringSlice{},
				EnvVars: []string{"SCOPES"},
			},
			&cli.StringSliceFlag{
				Name:    OriginsArg,
				Usage:   "Trusted Origins. At least 1 MUST be provided",
				Value:   &cli.StringSlice{},
				EnvVars: []string{"ORIGINS"},
			},
			&cli.StringFlag{
				Name:    DomainArg,
				Usage:   "Domain that the agent is running on.",
				Value:   "api.example.com",
				EnvVars: []string{"DOMAIN"},
			},
			&cli.StringFlag{
				Name:    BackendArg,
				Usage:   "Backend that we are proxying to.",
				Value:   "backend.example.com",
				EnvVars: []string{"BACKEND"},
			},
			&cli.StringFlag{
				Name:    CookieKeyArg,
				Usage:   "Key to the cookie jar.",
				Value:   "p2s5v8y/B?E(G+KbPeShVmYq3t6w9z$C",
				EnvVars: []string{"COOKIE_KEY"},
			},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(cCtx *cli.Context) error {
	debug := cCtx.Bool("debug")
	oidcProvider := cCtx.String(OidcProviderArg)
	clientID := cCtx.String(OidcClientIDArg)
	clientSecret := cCtx.String(OidcClientSecretArg)
	redirectURL := cCtx.String(RedirectURLArg)
	additionalScopes := cCtx.StringSlice(ScopesArg)
	origins := cCtx.StringSlice(OriginsArg)
	domain := cCtx.String(DomainArg)
	backend := cCtx.String(BackendArg)
	cookieKey := cCtx.String(CookieKeyArg)
	flow := cCtx.String(FlowArg)

	var logger *zap.Logger
	var err error
	if debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = logger.Sync()
	}()
	if len(origins) == 0 && flow != "device" {
		log.Fatal("at least 1 origin is required.")
	}
	scopes := []string{oidc.ScopeOpenID, "profile", "email"}
	scopes = append(scopes, additionalScopes...)

	auth, err := agent.NewOidcAgent(
		cCtx.Context, logger, oidcProvider,
		clientID, clientSecret, redirectURL,
		scopes, domain, origins, backend, cookieKey)
	if err != nil {
		log.Fatal(err)
	}
	var r *gin.Engine
	if flow == "authorization" {
		r = agent.NewCodeFlowRouter(auth)
	} else {
		r = agent.NewDeviceFlowRouter(auth)
	}

	return http.ListenAndServe("0.0.0.0:8080", r)
}
