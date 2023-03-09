package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	agent "github.com/nexodus-io/nexodus/pkg/oidcagent"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const (
	oidcProviderArg     = "oidc-provider"
	oidcBackChannelArg  = "oidc-backchannel"
	oidcClientIDArg     = "oidc-client-id"
	oidcClientSecretArg = "oidc-client-secret" // #nosec: G101
	insecureTLSArg      = "insecure-tls"
	redirectURLArg      = "redirect-url"
	scopesArg           = "scopes"
	originsArg          = "origins"
	domainArg           = "domain"
	backendArg          = "backend"
	cookieKeyArg        = "cookie-key"
	flowArg             = "flow"
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
				Name:  flowArg,
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
				Name:    oidcProviderArg,
				Usage:   "OIDC Provider URL",
				Value:   "https://accounts.google.com",
				EnvVars: []string{"OIDC_PROVIDER"},
			},
			&cli.StringFlag{
				Name:    oidcBackChannelArg,
				Usage:   "OIDC Backchannel URL",
				Value:   "https://auth.service.k8s.local",
				EnvVars: []string{"OIDC_BACKCHANNEL"},
			},
			&cli.BoolFlag{
				Name:    insecureTLSArg,
				Usage:   "Trust Any TLS Certificate",
				Value:   false,
				EnvVars: []string{"INSECURE_TLS"},
			},
			&cli.StringFlag{
				Name:    oidcClientIDArg,
				Usage:   "OIDC Client ID",
				Value:   "my-app-id",
				EnvVars: []string{"OIDC_CLIENT_ID"},
			},
			&cli.StringFlag{
				Name:    oidcClientSecretArg,
				Usage:   "OIDC Client Secret",
				Value:   "secret",
				EnvVars: []string{"OIDC_CLIENT_SECRET"},
			},
			&cli.StringFlag{
				Name:    redirectURLArg,
				Usage:   "Redirect URL. This is the URL of the SPA.",
				Value:   "https://example.com",
				EnvVars: []string{"REDIRECT_URL"},
			},
			&cli.StringSliceFlag{
				Name:    scopesArg,
				Usage:   "Additional OAUTH2 scopes",
				Value:   &cli.StringSlice{},
				EnvVars: []string{"SCOPES"},
			},
			&cli.StringSliceFlag{
				Name:    originsArg,
				Usage:   "Trusted Origins. At least 1 MUST be provided",
				Value:   &cli.StringSlice{},
				EnvVars: []string{"ORIGINS"},
			},
			&cli.StringFlag{
				Name:    domainArg,
				Usage:   "Domain that the agent is running on.",
				Value:   "api.example.com",
				EnvVars: []string{"DOMAIN"},
			},
			&cli.StringFlag{
				Name:    backendArg,
				Usage:   "Backend that we are proxying to.",
				Value:   "backend.example.com",
				EnvVars: []string{"BACKEND"},
			},
			&cli.StringFlag{
				Name:    cookieKeyArg,
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
	oidcProvider := cCtx.String(oidcProviderArg)
	oidcBackchannel := cCtx.String(oidcBackChannelArg)
	insecureTLS := cCtx.Bool(insecureTLSArg)
	clientID := cCtx.String(oidcClientIDArg)
	clientSecret := cCtx.String(oidcClientSecretArg)
	redirectURL := cCtx.String(redirectURLArg)
	additionalScopes := cCtx.StringSlice(scopesArg)
	origins := cCtx.StringSlice(originsArg)
	domain := cCtx.String(domainArg)
	backend := cCtx.String(backendArg)
	cookieKey := cCtx.String(cookieKeyArg)
	flow := cCtx.String(flowArg)

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
		oidcBackchannel, insecureTLS,
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

	// #nosec: G114
	return http.ListenAndServe("0.0.0.0:8080", r)
}
