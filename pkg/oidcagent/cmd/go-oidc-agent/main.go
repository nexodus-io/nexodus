package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	agent "github.com/nexodus-io/nexodus/pkg/oidcagent"
	"github.com/urfave/cli/v3"
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
	app := &cli.Command{
		Name: "go-oidc-agent",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Enable Debug Logging",
				Value:   false,
				Sources: cli.EnvVars("DEBUG"),
			},
			&cli.StringFlag{
				Name:  flowArg,
				Usage: "OAuth2 Flow",
				Value: "authorization",
				Action: func(ctx context.Context, command *cli.Command, s string) error {
					if s != "authorization" && s != "device" {
						return fmt.Errorf("flag 'flow' value should be one of 'authorization' or 'device'")
					}
					return nil
				},
				Sources: cli.EnvVars("OIDC_FLOW"),
			},
			&cli.StringFlag{
				Name:    oidcProviderArg,
				Usage:   "OIDC Provider URL",
				Value:   "https://accounts.google.com",
				Sources: cli.EnvVars("OIDC_PROVIDER"),
			},
			&cli.StringFlag{
				Name:    oidcBackChannelArg,
				Usage:   "OIDC Backchannel URL",
				Value:   "https://auth.service.k8s.local",
				Sources: cli.EnvVars("OIDC_BACKCHANNEL"),
			},
			&cli.BoolFlag{
				Name:    insecureTLSArg,
				Usage:   "Trust Any TLS Certificate",
				Value:   false,
				Sources: cli.EnvVars("INSECURE_TLS"),
			},
			&cli.StringFlag{
				Name:    oidcClientIDArg,
				Usage:   "OIDC Client ID",
				Value:   "my-app-id",
				Sources: cli.EnvVars("OIDC_CLIENT_ID"),
			},
			&cli.StringFlag{
				Name:    oidcClientSecretArg,
				Usage:   "OIDC Client Secret",
				Value:   "secret",
				Sources: cli.EnvVars("OIDC_CLIENT_SECRET"),
			},
			&cli.StringFlag{
				Name:    redirectURLArg,
				Usage:   "Redirect URL. This is the URL of the SPA.",
				Value:   "https://example.com",
				Sources: cli.EnvVars("REDIRECT_URL"),
			},
			&cli.StringSliceFlag{
				Name:    scopesArg,
				Usage:   "Additional OAUTH2 scopes",
				Sources: cli.EnvVars("SCOPES"),
			},
			&cli.StringSliceFlag{
				Name:    originsArg,
				Usage:   "Trusted Origins. At least 1 MUST be provided",
				Sources: cli.EnvVars("ORIGINS"),
			},
			&cli.StringFlag{
				Name:    domainArg,
				Usage:   "Domain that the agent is running on.",
				Value:   "api.example.com",
				Sources: cli.EnvVars("DOMAIN"),
			},
			&cli.StringFlag{
				Name:    backendArg,
				Usage:   "Backend that we are proxying to.",
				Value:   "backend.example.com",
				Sources: cli.EnvVars("BACKEND"),
			},
			&cli.StringFlag{
				Name:    cookieKeyArg,
				Usage:   "Key to the cookie jar.",
				Value:   "p2s5v8y/B?E(G+KbPeShVmYq3t6w9z$C",
				Sources: cli.EnvVars("COOKIE_KEY"),
			},
		},
		Action: run,
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, command *cli.Command) error {
	debug := command.Bool("debug")
	oidcProvider := command.String(oidcProviderArg)
	oidcBackchannel := command.String(oidcBackChannelArg)
	insecureTLS := command.Bool(insecureTLSArg)
	clientID := command.String(oidcClientIDArg)
	clientSecret := command.String(oidcClientSecretArg)
	redirectURL := command.String(redirectURLArg)
	additionalScopes := command.StringSlice(scopesArg)
	origins := command.StringSlice(originsArg)
	domain := command.String(domainArg)
	backend := command.String(backendArg)
	cookieKey := command.String(cookieKeyArg)
	flow := command.String(flowArg)

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
		ctx, logger, oidcProvider,
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
