package agent

import (
	"context"
	"log"
	"net/url"

	"github.com/coreos/go-oidc"
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
	provider       OpenIDConnectProvider
	verifier       IDTokenVerifier
	endSessionURL  string
	backend        *url.URL
	cookieKey      string
}

type OauthConfig interface {
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource
}

type OpenIDConnectProvider interface {
	UserInfo(ctx context.Context, tokenSource oauth2.TokenSource) (*oidc.UserInfo, error)
}

type IDTokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

func NewOidcAgent(ctx context.Context,
	logger *zap.Logger,
	oidcProvider string,
	clientID string,
	clientSecret string,
	redirectURL string,
	scopes []string,
	domain string,
	origins []string,
	backend string,
	cookieKey string,
) (*OidcAgent, error) {
	provider, err := oidc.NewProvider(ctx, oidcProvider)
	if err != nil {
		log.Fatal(err)
	}

	var claims struct {
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
		provider:       provider,
		verifier:       verifier,
		endSessionURL:  claims.EndSessionURL,
		backend:        backendURL,
		cookieKey:      cookieKey,
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
