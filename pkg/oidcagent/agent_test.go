package agent

import (
	"context"
	"testing"

	"github.com/coreos/go-oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type FakeOauthConfig struct {
	AuthCodeURLFn func(state string, opts ...oauth2.AuthCodeOption) string
	ExchangeFn    func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	TokenSourceFn func(ctx context.Context, t *oauth2.Token) oauth2.TokenSource
}

func (f *FakeOauthConfig) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return f.AuthCodeURLFn(state, opts...)
}

func (f *FakeOauthConfig) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return f.ExchangeFn(ctx, code, opts...)
}

func (f *FakeOauthConfig) TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource {
	return f.TokenSourceFn(ctx, t)
}

type FakeOpenIDConnectProvider struct {
	EndpointFn func() oauth2.Endpoint
	UserInfoFn func(ctx context.Context, tokenSource oauth2.TokenSource) (*oidc.UserInfo, error)
}

func (f *FakeOpenIDConnectProvider) Endpoint() oauth2.Endpoint {
	return f.EndpointFn()
}

func (f *FakeOpenIDConnectProvider) UserInfo(ctx context.Context, tokenSource oauth2.TokenSource) (*oidc.UserInfo, error) {
	return f.UserInfoFn(ctx, tokenSource)
}

type FakeIDTokenVerifier struct {
	VerifyFn func(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

func (f *FakeIDTokenVerifier) Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	return f.VerifyFn(ctx, rawIDToken)
}

func TestLogoutURL(t *testing.T) {
	o := OidcAgent{
		clientID:      "test-client",
		redirectURL:   "https://example.com",
		endSessionURL: "https://auth.example.com/logout",
	}
	actual, err := o.LogoutURL("my-id-token")
	require.NoError(t, err)
	expected := "https://auth.example.com/logout?client_id=test-client&id_token_hint=my-id-token&post_logout_redirect_uri=https%3A%2F%2Fexample.com"
	assert.Equal(t, expected, actual.String())
}
