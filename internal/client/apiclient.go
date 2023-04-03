package client

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
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

	clientConfig := public.NewConfiguration()
	clientConfig.HTTPClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: opts.tlsConfig,
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

	clientConfig.HTTPClient = config.Client(ctx, token)
	return public.NewAPIClient(clientConfig), nil
}
