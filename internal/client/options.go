package client

import (
	"crypto/tls"
	"golang.org/x/oauth2"
)

type options struct {
	deviceFlow   bool
	clientSecret string
	username     string
	password     string
	tokenStore   TokenStore
	tlsConfig    *tls.Config
	bearerToken  string
	userAgent    string
}

type TokenStore interface {
	Load() (*oauth2.Token, error)
	Store(*oauth2.Token) error
}

func newOptions(opts ...Option) (*options, error) {
	o := &options{}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

type Option func(o *options) error

func WithPasswordGrant(
	username string,
	password string,
) Option {
	return func(o *options) error {
		o.deviceFlow = false
		o.username = username
		o.password = password
		return nil
	}
}

func WithUserAgent(
	userAgent string,
) Option {
	return func(o *options) error {
		o.userAgent = userAgent
		return nil
	}
}

func WithBearerToken(
	bearerToken string,
) Option {
	return func(o *options) error {
		o.bearerToken = bearerToken
		return nil
	}
}
func WithTLSConfig(
	config *tls.Config,
) Option {
	return func(o *options) error {
		o.tlsConfig = config
		return nil
	}
}

func WithDeviceFlow() Option {
	return func(o *options) error {
		o.deviceFlow = true
		return nil
	}
}

func WithTokenStore(
	tokenStore TokenStore,
) Option {
	return func(o *options) error {
		o.tokenStore = tokenStore
		return nil
	}
}
