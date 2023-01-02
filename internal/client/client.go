package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Client struct {
	options *options
	baseURL *url.URL
	client  *http.Client
}

func NewClient(ctx context.Context, addr string, options ...Option) (*Client, error) {
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

	resp, err := startLogin(*baseURL)
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
		Scopes:       []string{"openid", "profile", "email"},
	}

	var token *oauth2.Token
	var rawIdToken interface{}
	if c.options.deviceFlow {
		token, rawIdToken, err = newDeviceFlowToken(ctx, resp.DeviceAuthURL, provider.Endpoint().TokenURL, resp.ClientID)
		if err != nil {
			return nil, err
		}
	} else if c.options.token == "" {
		token, err = config.PasswordCredentialsToken(ctx, c.options.username, c.options.password)
		if err != nil {
			return nil, err
		}
		rawIdToken = token.Extra("id_token")
	} else {
		if err = json.Unmarshal([]byte(c.options.token), &token); err != nil {
			return nil, err
		}
		var tokenRaw map[string]interface{}
		if err = json.Unmarshal([]byte(c.options.token), &tokenRaw); err != nil {
			return nil, err
		}
		rawIdToken = tokenRaw["id_token"]
	}
	if rawIdToken == nil {
		return nil, fmt.Errorf("no id_token in response")
	}

	if _, err = verifier.Verify(ctx, rawIdToken.(string)); err != nil {
		return nil, err
	}

	c.client = config.Client(ctx, token)

	return &c, nil
}
