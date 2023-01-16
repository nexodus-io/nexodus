package agent

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type OidcAgent struct {
	logger         *zap.SugaredLogger
	domain         string
	trustedOrigins []string
	clientID       string
	redirectURL    string
	oauthConfig    OauthConfig
	oidcIssuer     string
	provider       OpenIDConnectProvider
	verifier       IDTokenVerifier
	endSessionURL  string
	deviceAuthURL  string
	backend        *url.URL
	cookieKey      string
	insecureTLS    bool
}

type OauthConfig interface {
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource
}

type OpenIDConnectProvider interface {
	Endpoint() oauth2.Endpoint
	UserInfo(ctx context.Context, tokenSource oauth2.TokenSource) (*oidc.UserInfo, error)
}

type IDTokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

func NewOidcAgent(ctx context.Context,
	logger *zap.Logger,
	oidcProvider string,
	oidcBackchannel string,
	insecureTLS bool,
	clientID string,
	clientSecret string,
	redirectURL string,
	scopes []string,
	domain string,
	origins []string,
	backend string,
	cookieKey string,
) (*OidcAgent, error) {
	if insecureTLS {
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: transport}
		ctx = oidc.ClientContext(ctx, client)
	}

	var provider *oidc.Provider
	var err error
	if oidcBackchannel != "" {
		ctx = oidc.InsecureIssuerURLContext(ctx,
			oidcProvider,
		)
		provider, err = oidc.NewProvider(ctx, oidcBackchannel)

	} else {
		provider, err = oidc.NewProvider(ctx, oidcProvider)
	}
	if err != nil {
		log.Fatal(err)
	}

	var claims struct {
		DeviceAuthURL string `json:"device_authorization_endpoint"`
		EndSessionURL string `json:"end_session_endpoint"`
	}
	err = provider.Claims(&claims)
	if err != nil {
		log.Fatal(err)
	}

	oidcConfig := &oidc.Config{
		ClientID: clientID,
	}
	verifier := provider.Verifier(oidcConfig)

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       scopes,
	}

	backendURL, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}

	auth := &OidcAgent{
		logger:         logger.Sugar(),
		domain:         domain,
		trustedOrigins: origins,
		clientID:       clientID,
		redirectURL:    redirectURL,
		oauthConfig:    config,
		oidcIssuer:     oidcProvider,
		provider:       provider,
		verifier:       verifier,
		endSessionURL:  claims.EndSessionURL,
		deviceAuthURL:  claims.DeviceAuthURL,
		backend:        backendURL,
		cookieKey:      cookieKey,
		insecureTLS:    insecureTLS,
	}
	return auth, nil
}

func (o *OidcAgent) LogoutURL(idToken string) (*url.URL, error) {
	u, err := url.Parse(o.endSessionURL)
	if err != nil {
		return nil, err
	}
	params := u.Query()
	params.Add("client_id", o.clientID)
	params.Add("id_token_hint", idToken)
	params.Add("post_logout_redirect_uri", o.redirectURL)
	u.RawQuery = params.Encode()
	return u, nil
}
