package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type RoundTripperFunc func(req *http.Request) (*http.Response, error)

func (fn RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func NewClient(ctx context.Context, addr string, authcb func(string), options ...Option) (*APIClient, error) {
	opts, err := newOptions(options...)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	clientConfig := NewConfiguration()
	clientConfig.HTTPClient = &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
			TLSClientConfig:       opts.tlsConfig,
		},
	}
	clientConfig.Host = baseURL.Host
	clientConfig.Scheme = baseURL.Scheme
	if opts.userAgent != "" {
		clientConfig.UserAgent = opts.userAgent
	}
	if opts.bearerToken != "" {
		nextTransport := clientConfig.HTTPClient.Transport
		clientConfig.HTTPClient.Transport = RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", opts.bearerToken))
			return nextTransport.RoundTrip(req)
		})
	} else {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, clientConfig.HTTPClient)
		apiClient := NewAPIClient(clientConfig)
		clientConfig.HTTPClient, err = createOAuthHttpClient(ctx, apiClient, opts, authcb)
		if err != nil {
			return nil, err
		}
	}
	return NewAPIClient(clientConfig), nil
}

func createOAuthHttpClient(ctx context.Context, apiClient *APIClient, opts *options, authcb func(string)) (*http.Client, error) {
	startTime := time.Now()
	resp, _, err := apiClient.AuthApi.DeviceStart(ctx).Execute()
	if err != nil {
		return nil, err
	}

	// knowing the right time is needed to validate if the JWT has expired
	serverTimeFn := time.Now

	// Can we use the server's reported time to detect if our time is skewed?
	if !resp.GetServerTime().IsZero() {

		// account for the time spend sending the request to the server...
		rtt := time.Since(startTime)
		timeSkew := resp.ServerTime.Sub(startTime.Add(rtt / 2))

		timeSkewGrace := 5 * time.Second
		if timeSkew > timeSkewGrace || timeSkew < -timeSkewGrace {
			// adjust for the time skew...
			serverTimeFn = func() time.Time {
				return time.Now().Add(timeSkew)
			}
		}
	}

	provider, err := oidc.NewProvider(ctx, resp.GetIssuer())
	if err != nil {
		return nil, err
	}

	oidcConfig := &oidc.Config{
		ClientID: resp.GetClientId(),
		Now:      serverTimeFn,
	}

	verifier := provider.Verifier(oidcConfig)

	config := &oauth2.Config{
		ClientID:     resp.GetClientId(),
		ClientSecret: opts.clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{"openid", "profile", "email", "offline_access", "read:organizations", "write:organizations", "read:users", "write:users", "read:devices", "write:devices"},
	}

	var token *oauth2.Token
	var rawIdToken interface{}
	if opts.tokenStore != nil {
		// attempt to load the token...
		token, _ = opts.tokenStore.Load()
	}
	if token == nil {
		if opts.deviceFlow {
			token, rawIdToken, err = newDeviceFlowToken(ctx, resp.GetDeviceAuthorizationEndpoint(), provider.Endpoint().TokenURL, resp.GetClientId(), authcb)
			if err != nil {
				return nil, err
			}
		} else if opts.username != "" && opts.password != "" {
			token, err = config.PasswordCredentialsToken(ctx, opts.username, opts.password)
			if err != nil {
				return nil, err
			}
			rawIdToken = token.Extra("id_token")
		} else {
			return nil, fmt.Errorf("no authentication method provided")
		}
		if rawIdToken == nil {
			return nil, fmt.Errorf("no id_token in response")
		}
		if _, err = verifier.Verify(ctx, rawIdToken.(string)); err != nil {
			return nil, err
		}

		if opts.tokenStore != nil {
			err = opts.tokenStore.Store(token)
			if err != nil {
				return nil, err
			}
		}
	}

	var source = config.TokenSource(ctx, token)
	if opts.tokenStore != nil {
		source = oauth2.ReuseTokenSource(token, &storeOnChangeSource{
			tokenStore: opts.tokenStore,
			source:     source,
		})
	}

	return oauth2.NewClient(ctx, source), nil
}

type storeOnChangeSource struct {
	tokenStore TokenStore
	source     oauth2.TokenSource
	lastToken  *oauth2.Token
}

var _ oauth2.TokenSource = &storeOnChangeSource{}

func (s *storeOnChangeSource) Token() (*oauth2.Token, error) {
	next, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	if next != s.lastToken {
		s.lastToken = next
		err = s.tokenStore.Store(next)
		if err != nil {
			return nil, err
		}
	}
	return next, nil
}
