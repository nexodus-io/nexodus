package client_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/go-jose/go-jose/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/pkg/oidcagent/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type testTokenStore struct {
	token *oauth2.Token
}

func (s *testTokenStore) Load() (*oauth2.Token, error) {
	return s.token, nil
}

func (s *testTokenStore) Store(token *oauth2.Token) error {
	s.token = token
	return nil
}

func TestWithTokenFileOption(t *testing.T) {

	require := require.New(t)
	assert := assert.New(t)

	mockRouter := http.NewServeMux()
	mockServer := httptest.NewServer(mockRouter)
	defer mockServer.Close()

	accesTokensCreated := int64(0)
	err := addMockOIDCRoutes(mockServer, mockRouter, func() string {
		// this gets called to create an access token
		return fmt.Sprintf("%d", atomic.AddInt64(&accesTokensCreated, 1))
	})
	require.NoError(err)

	_ = os.Mkdir("./tmp", 0755)
	// #nosec G101
	tokenFile := "./tmp/token.json"
	_ = os.Remove(tokenFile)

	store := &testTokenStore{}
	assert.Equal(int64(0), accesTokensCreated)
	_, err = client.NewAPIClient(context.Background(), mockServer.URL, nil,
		client.WithPasswordGrant("fake", "password"),
		client.WithTokenStore(store),
	)
	require.NoError(err)
	assert.Equal(int64(1), accesTokensCreated)

	// create the client again... this time it should re-use the token stored in the file..
	c, err := client.NewAPIClient(context.Background(), mockServer.URL, nil,
		client.WithPasswordGrant("fake", "password"),
		client.WithTokenStore(store),
	)
	// token endpoint should not have been hit.
	require.NoError(err)
	assert.Equal(int64(1), accesTokensCreated)
	tokenData1, err := os.ReadFile(tokenFile)
	require.NoError(err)

	// wait for the token to expire...
	mockRouter.HandleFunc("/api/users/me", func(resp http.ResponseWriter, request *http.Request) {
		sendJson(resp, 200, "{}")
	})
	time.Sleep(time.Second * 3)
	_, _, err = c.UsersApi.GetUser(context.Background(), "me").Execute()
	assert.NoError(err)
	assert.Equal(int64(2), accesTokensCreated)
	tokenData2, err := os.ReadFile(tokenFile)
	assert.NoError(err)
	assert.NotEqual(string(tokenData1), string(tokenData2))

}

func sendJson(resp http.ResponseWriter, status int, body interface{}) {
	resp.Header().Add("Content-Type", "application/json")
	resp.WriteHeader(status)
	if body != nil {
		switch body := body.(type) {
		case string:
			_, _ = resp.Write([]byte(body))
		default:
			_ = json.NewEncoder(resp).Encode(body)
		}
	}
}
func addMockOIDCRoutes(server *httptest.Server, router *http.ServeMux, createAccessToken func() string) error {
	router.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Println("not found:", req.URL.Path)
		sendJson(resp, http.StatusNotFound, nil)
	})
	router.HandleFunc("/device/login/start", func(resp http.ResponseWriter, req *http.Request) {
		sendJson(resp, http.StatusOK, models.DeviceStartResponse{
			ClientID:      "nexodus-cli",
			DeviceAuthURL: server.URL + "/realms/nexodus/protocol/openid-connect/auth/device",
			Issuer:        server.URL + "/realms/nexodus",
		})
	})
	router.HandleFunc("/realms/nexodus/.well-known/openid-configuration", func(resp http.ResponseWriter, req *http.Request) {
		sendJson(resp, http.StatusOK, struct {
			Issuer      string   `json:"issuer"`
			AuthURL     string   `json:"authorization_endpoint"`
			TokenURL    string   `json:"token_endpoint"`
			JWKSURL     string   `json:"jwks_uri"`
			UserInfoURL string   `json:"userinfo_endpoint"`
			Algorithms  []string `json:"id_token_signing_alg_values_supported"`
		}{
			Issuer:      server.URL + "/realms/nexodus",
			AuthURL:     server.URL + "/realms/nexodus/protocol/openid-connect/auth",
			TokenURL:    server.URL + "/realms/nexodus/protocol/openid-connect/token",
			JWKSURL:     server.URL + "/realms/nexodus/protocol/openid-connect/certs",
			UserInfoURL: server.URL + "/realms/nexodus/protocol/openid-connect/userinfo",
			Algorithms:  []string{"RS256"},
		})
	})
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	err = privateKey.Validate()
	if err != nil {
		return err
	}
	router.HandleFunc("/realms/nexodus/protocol/openid-connect/certs", func(resp http.ResponseWriter, req *http.Request) {
		set := jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{{
				KeyID:     "test",
				Algorithm: "RS256",
				Use:       "sig",
				Key:       &privateKey.PublicKey,
			}},
		}
		x, err := json.Marshal(set)
		if err != nil {
			panic(err)
		}
		sendJson(resp, http.StatusOK, string(x))
	})
	router.HandleFunc("/realms/nexodus/protocol/openid-connect/token", func(resp http.ResponseWriter, req *http.Request) {
		claims := jwt.MapClaims{}
		claims["authorized"] = true
		claims["sub"] = "test"
		claims["iss"] = server.URL + "/realms/nexodus"
		claims["aud"] = "nexodus-cli"
		claims["exp"] = time.Now().Add(2 * time.Second).UnixMilli()
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		idToken, err := token.SignedString(privateKey)
		if err != nil {
			panic(err)
		}
		sendJson(resp, http.StatusOK, map[string]interface{}{
			"id_token":           idToken,
			"access_token":       createAccessToken(),
			"refresh_token":      "eyJhbGciOiJIUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJlOGRiMjNiYi05MDU4LTRiZDctYWM1MS00OTYwYWY1ZjRhMzYifQ.eyJpYXQiOjE2ODA3OTIyMDgsImp0aSI6ImZmNTRkNTFjLThlODctNDMzZi05OTA5LWVmY2Q2YTlmNWViZCIsImlzcyI6Imh0dHBzOi8vYXV0aC50cnkubmV4b2R1cy4xMjcuMC4wLjEubmlwLmlvL3JlYWxtcy9uZXhvZHVzIiwiYXVkIjoiaHR0cHM6Ly9hdXRoLnRyeS5uZXhvZHVzLjEyNy4wLjAuMS5uaXAuaW8vcmVhbG1zL25leG9kdXMiLCJzdWIiOiIwMTU3OGM5ZS04ZTc2LTQ2YTQtYjJiMi01MDc4OGNlYzJjY2QiLCJ0eXAiOiJPZmZsaW5lIiwiYXpwIjoibmV4b2R1cy1jbGkiLCJzZXNzaW9uX3N0YXRlIjoiNjMyNzEwMzktZDRmMy00YjY4LWJjNzUtYmU1MTMyODFjYzlkIiwic2NvcGUiOiJvcGVuaWQgd3JpdGU6ZGV2aWNlcyByZWFkOnVzZXJzIHdyaXRlOm9yZ2FuaXphdGlvbnMgd3JpdGU6dXNlcnMgcmVhZDpvcmdhbml6YXRpb25zIHJlYWQ6ZGV2aWNlcyBwcm9maWxlIG9mZmxpbmVfYWNjZXNzIGVtYWlsIiwic2lkIjoiNjMyNzEwMzktZDRmMy00YjY4LWJjNzUtYmU1MTMyODFjYzlkIn0.63oUnf6PDzudiF7XOkeK5_FyZ4DXprHPNnknfosCEAQ",
			"token_type":         "Bearer",
			"expires_in":         2,
			"refresh_expires_in": 0,
			"not-before-policy":  0,
			"session_state":      "63271039-d4f3-4b68-bc75-be513281cc9d",
			"scope":              "openid write:devices read:users write:organizations write:users read:organizations read:devices profile offline_access email",
		})
	})
	return err
}
