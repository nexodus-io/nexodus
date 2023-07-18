package handlers

import (
	"context"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/nexodus-io/nexodus/pkg/oidcagent"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"net/http"
	"net/http/httptest"
	"net/url"
)

const SESSION_ID_COOKIE_NAME = "sid"

// Check implements Envoy Authorization service. Proto file:
// https://github.com/envoyproxy/envoy/blob/main/api/envoy/service/auth/v3/external_auth.proto
//
// We use this to convert the browser cookie to a JWT in the authorization header.  This can then be
// used by envoy to rate limit requests.
func (api *API) Check(ctx context.Context, checkReq *auth.CheckRequest) (*auth.CheckResponse, error) {

	okResponse := &auth.CheckResponse{
		Status: &status.Status{Code: int32(codes.OK)},
		HttpResponse: &auth.CheckResponse_OkResponse{
			OkResponse: &auth.OkHttpResponse{},
		},
	}

	if checkReq.Attributes.Request.Http.Headers["authorization"] != "" {
		return okResponse, nil
	}

	// Can use a cookie to get the authorization header?
	req := &http.Request{
		URL:    &url.URL{},
		Header: map[string][]string{},
	}
	for k, v := range checkReq.Attributes.Request.Http.Headers {
		if k == "cookie" {
			k = "Cookie"
		}
		req.Header[k] = []string{v}
	}
	resp := &httptest.ResponseRecorder{}

	session, err := api.sessionManager.Start(ctx, resp, req)
	if err != nil {
		return okResponse, nil
	}

	tokenRaw, ok := session.Get(oidcagent.TokenKey)
	if !ok {
		return okResponse, nil
	}
	token, err := oidcagent.JsonStringToToken(tokenRaw.(string))
	if err != nil {
		return okResponse, nil
	}

	// add the access token header to the upstream requests..
	return &auth.CheckResponse{
		Status: &status.Status{Code: int32(codes.OK)},
		HttpResponse: &auth.CheckResponse_OkResponse{
			OkResponse: &auth.OkHttpResponse{
				Headers: []*core.HeaderValueOption{
					{
						Header: &core.HeaderValue{
							Key:   "authorization",
							Value: "Bearer " + token.AccessToken,
						},
					},
				},
			},
		},
	}, nil
}
