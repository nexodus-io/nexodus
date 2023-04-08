package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"golang.org/x/oauth2"
)

type APIClient = public.APIClient

func NewAPIClient(ctx context.Context, addr string, authcb func(string), options ...Option) (*APIClient, error) {
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
	clientConfig := public.NewConfiguration()
	clientConfig.HTTPClient = &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       opts.tlsConfig,
		},
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, clientConfig.HTTPClient)

	clientConfig.Host = baseURL.Host
	clientConfig.Scheme = baseURL.Scheme

	apiClient := public.NewAPIClient(clientConfig)

	resp, _, err := apiClient.AuthApi.DeviceStart(ctx).Execute()
	if err != nil {
		return nil, err
	}

	provider, err := oidc.NewProvider(ctx, resp.Issuer)
	if err != nil {
		return nil, err
	}

	oidcConfig := &oidc.Config{
		ClientID: resp.ClientId,
	}

	verifier := provider.Verifier(oidcConfig)

	config := &oauth2.Config{
		ClientID:     resp.ClientId,
		ClientSecret: opts.clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{"openid", "profile", "email", "offline_access", "read:organizations", "write:organizations", "read:users", "write:users", "read:devices", "write:devices"},
	}

	var token *oauth2.Token
	var rawIdToken interface{}
	if opts.tokenFile != "" {
		// attempt to load the token file..
		token, _ = loadTokenFromFile(opts.tokenFile)
	}
	if token == nil {
		if opts.deviceFlow {
			token, rawIdToken, err = newDeviceFlowToken(ctx, resp.DeviceAuthorizationEndpoint, provider.Endpoint().TokenURL, resp.ClientId, authcb)
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

		if opts.tokenFile != "" {
			err = saveTokenToFile(opts.tokenFile, token)
			if err != nil {
				return nil, err
			}
		}
	}

	var source = config.TokenSource(ctx, token)
	if opts.tokenFile != "" {
		source = oauth2.ReuseTokenSource(token, &storeOnChangeSource{
			file:   opts.tokenFile,
			source: source,
		})
	}

	clientConfig.HTTPClient = oauth2.NewClient(ctx, source)
	return public.NewAPIClient(clientConfig), nil
}

type storeOnChangeSource struct {
	file      string
	source    oauth2.TokenSource
	lastToken *oauth2.Token
}

var _ oauth2.TokenSource = &storeOnChangeSource{}

func (s *storeOnChangeSource) Token() (*oauth2.Token, error) {
	next, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	if next != s.lastToken {
		s.lastToken = next
		err = saveTokenToFile(s.file, next)
		if err != nil {
			return nil, err
		}
	}
	return next, nil
}

func loadTokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveTokenToFile(path string, token *oauth2.Token) error {
	// Create the path to the file if it doesn't exist.
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0600); err != nil {
			return err
		}
	}
	// Save the token to a file at the given path.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
