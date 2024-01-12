package handlers

import (
	"context"
	"encoding/json"
	"errors"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/pkg/oidcagent"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

	// is there already an authorization header?
	authorizationHeader := checkReq.Attributes.Request.Http.Headers["authorization"]
	if authorizationHeader != "" {

		// Does it look like a reg key?
		if strings.HasPrefix(authorizationHeader, "Bearer RK:") {
			token := strings.TrimPrefix(authorizationHeader, "Bearer ")
			return checkRegistrationToken(ctx, api, token)
		} else if strings.HasPrefix(authorizationHeader, "Bearer DT:") {
			token := strings.TrimPrefix(authorizationHeader, "Bearer ")
			return checkDeviceToken(ctx, api, token)
		} else if strings.HasPrefix(authorizationHeader, "Bearer ST:") {
			token := strings.TrimPrefix(authorizationHeader, "Bearer ")
			return checkSiteToken(ctx, api, token)
		}
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

func checkRegistrationToken(ctx context.Context, api *API, token string) (*auth.CheckResponse, error) {
	var regToken models.RegKey
	db := api.db.WithContext(ctx)
	result := db.First(&regToken, "bearer_token = ?", token)
	if result.Error != nil {

		message := "internal server error"
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			message = "invalid reg key"
		}
		return denyCheckResponse(401, models.NewBaseError(message))
	}

	var user models.User
	result = db.First(&user, "id = ?", regToken.OwnerID)
	if result.Error != nil {

		message := "internal server error"
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			message = "invalid reg key user"
		}
		return denyCheckResponse(401, models.NewBaseError(message))
	}

	// replace it with a JWT token...
	claims := models.NexodusClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  api.URL,
			ID:      regToken.ID.String(),
			Subject: user.IdpID,
		},
		VpcID: regToken.VpcID,
		Scope: "reg-token",
	}
	if regToken.DeviceId != nil {
		claims.DeviceID = *regToken.DeviceId
	}
	if regToken.ExpiresAt != nil {
		claims.ExpiresAt = jwt.NewNumericDate(*regToken.ExpiresAt)
	}

	jwttoken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(api.PrivateKey)
	if err != nil {
		return denyCheckResponse(401, models.NewBaseError("internal server error"))
	}

	return &auth.CheckResponse{
		Status: &status.Status{Code: int32(codes.OK)},
		HttpResponse: &auth.CheckResponse_OkResponse{
			OkResponse: &auth.OkHttpResponse{
				Headers: []*core.HeaderValueOption{
					{
						AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						Header: &core.HeaderValue{
							Key:   "authorization",
							Value: "Bearer " + jwttoken,
						},
					},
				},
			},
		},
	}, nil
}

func checkSiteToken(ctx context.Context, api *API, token string) (*auth.CheckResponse, error) {

	var site models.Site
	db := api.db.WithContext(ctx)
	result := db.First(&site, "bearer_token = ?", token)
	if result.Error != nil {
		message := "internal server error"
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			message = "invalid site token"
		}
		return denyCheckResponse(401, models.NewBaseError(message))
	}

	var user models.User
	result = db.First(&user, "id = ?", site.OwnerID)
	if result.Error != nil {

		message := "internal server error"
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			message = "invalid reg key user"
		}
		return denyCheckResponse(401, models.NewBaseError(message))
	}

	// replace it with a JWT token...
	claims := models.NexodusClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  api.URL,
			ID:      site.ID.String(),
			Subject: user.IdpID,
		},
		VpcID: site.VpcID,
		Scope: "device-token",
	}

	jwttoken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(api.PrivateKey)
	if err != nil {
		return denyCheckResponse(401, models.NewBaseError("internal server error"))
	}

	return &auth.CheckResponse{
		Status: &status.Status{Code: int32(codes.OK)},
		HttpResponse: &auth.CheckResponse_OkResponse{
			OkResponse: &auth.OkHttpResponse{
				Headers: []*core.HeaderValueOption{
					{
						AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						Header: &core.HeaderValue{
							Key:   "authorization",
							Value: "Bearer " + jwttoken,
						},
					},
				},
			},
		},
	}, nil

}

func checkDeviceToken(ctx context.Context, api *API, token string) (*auth.CheckResponse, error) {
	var device models.Device
	db := api.db.WithContext(ctx)
	result := db.First(&device, "bearer_token = ?", token)
	if result.Error != nil {
		message := "internal server error"
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			message = "invalid device token"
		}
		return denyCheckResponse(401, models.NewBaseError(message))
	}

	var user models.User
	result = db.First(&user, "id = ?", device.OwnerID)
	if result.Error != nil {

		message := "internal server error"
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			message = "invalid reg key user"
		}
		return denyCheckResponse(401, models.NewBaseError(message))
	}

	// replace it with a JWT token...
	claims := models.NexodusClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  api.URL,
			ID:      device.ID.String(),
			Subject: user.IdpID,
		},
		VpcID: device.VpcID,
		Scope: "device-token",
	}

	jwttoken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(api.PrivateKey)
	if err != nil {
		return denyCheckResponse(401, models.NewBaseError("internal server error"))
	}

	return &auth.CheckResponse{
		Status: &status.Status{Code: int32(codes.OK)},
		HttpResponse: &auth.CheckResponse_OkResponse{
			OkResponse: &auth.OkHttpResponse{
				Headers: []*core.HeaderValueOption{
					{
						AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						Header: &core.HeaderValue{
							Key:   "authorization",
							Value: "Bearer " + jwttoken,
						},
					},
				},
			},
		},
	}, nil

}

func denyCheckResponse(statusCode int, baseError models.BaseError) (*auth.CheckResponse, error) {
	data, err := json.Marshal(baseError)
	if err != nil {
		return nil, err
	}
	return &auth.CheckResponse{
		Status: &status.Status{Code: int32(codes.PermissionDenied)},
		HttpResponse: &auth.CheckResponse_DeniedResponse{
			DeniedResponse: &auth.DeniedHttpResponse{
				Headers: []*core.HeaderValueOption{
					{
						AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						Header: &core.HeaderValue{
							Key:   "Content-Type",
							Value: "application/json",
						},
					},
				},
				Status: &v3.HttpStatus{Code: v3.StatusCode(statusCode)},
				Body:   string(data),
			},
		},
	}, nil
}
