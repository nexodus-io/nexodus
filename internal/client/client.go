package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type Client struct {
	logger  *zap.SugaredLogger
	options *options
	baseURL *url.URL
	client  *http.Client
}

func NewClient(ctx context.Context, addr string, authcb func(string), options ...Option) (*Client, error) {
	opts, err := newOptions(options...)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	c := Client{
		options: opts,
		baseURL: baseURL,
	}

	if c.options.logger != nil {
		c.logger = c.options.logger
	} else {
		l, err := zap.NewDevelopment()
		if err != nil {
			return nil, err
		}
		c.logger = l.Sugar()
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: opts.tlsConfig,
		},
	}

	resp, err := startLogin(httpClient, *baseURL)
	if err != nil {
		return nil, err
	}

	provider, err := oidc.NewProvider(ctx, resp.Issuer)
	if err != nil {
		return nil, err
	}

	oidcConfig := &oidc.Config{
		ClientID: resp.ClientID,
	}

	verifier := provider.Verifier(oidcConfig)

	config := &oauth2.Config{
		ClientID:     resp.ClientID,
		ClientSecret: c.options.clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{"openid", "profile", "email", "offline_access", "read:organizations", "write:organizations", "read:users", "write:users", "read:devices", "write:devices"},
	}

	var token *oauth2.Token
	var rawIdToken interface{}
	if c.options.deviceFlow {
		token, rawIdToken, err = newDeviceFlowToken(ctx, resp.DeviceAuthURL, provider.Endpoint().TokenURL, resp.ClientID, authcb)
		if err != nil {
			return nil, err
		}
	} else if c.options.username != "" && c.options.password != "" {
		token, err = config.PasswordCredentialsToken(ctx, c.options.username, c.options.password)
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

	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	c.client = config.Client(ctx, token)

	return &c, nil
}
