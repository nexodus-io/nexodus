package main

import (
	"log"
	"net/http"
	"os"

	"github.com/coreos/go-oidc"
	agent "github.com/redhat-et/go-oidc-agent"
	"github.com/urfave/cli/v2"
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
)

func main() {
	app := &cli.App{
		Name: "go-oidc-agent",
		Flags: []cli.Flag{
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
				Usage:   "OIDC Provider URL",
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
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(cCtx *cli.Context) error {
	oidcProvider := cCtx.String(OidcProviderArg)
	clientID := cCtx.String(OidcClientIDArg)
	clientSecret := cCtx.String(OidcClientSecretArg)
	redirectURL := cCtx.String(RedirectURLArg)
	additionalScopes := cCtx.StringSlice(ScopesArg)
	origins := cCtx.StringSlice(OriginsArg)
	domain := cCtx.String(DomainArg)
	backend := cCtx.String(BackendArg)

	if len(origins) == 0 {
		log.Fatal("at least 1 origin is required.")
	}
	scopes := []string{oidc.ScopeOpenID, "profile", "email"}
	scopes = append(scopes, additionalScopes...)

	auth, err := agent.NewOidcAgent(cCtx.Context, oidcProvider, clientID, clientSecret, redirectURL, scopes, domain, origins, backend)
	if err != nil {
		log.Fatal(err)
	}
	r := agent.NewRouter(auth)
	return http.ListenAndServe("0.0.0.0:8080", r)
}
