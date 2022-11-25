package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/coreos/go-oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

func TestLoginStart(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
		oauthConfig: &FakeOauthConfig{
			AuthCodeURLFn: func(state string, opts ...oauth2.AuthCodeOption) string {
				return "https://auth.example.com/auth?state=foo&client_id=bar"
			},
		},
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login/start", auth.LoginStart)
	req, _ := http.NewRequest("POST", "/login/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	data, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var response LoginStartReponse
	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.AuthorizationRequestURL)

	cookies := w.Result().Cookies()
	assert.Equal(t, 2, len(cookies))
	assert.Equal(t, "state", cookies[0].Name)
	assert.Equal(t, "nonce", cookies[1].Name)
}

func TestLoginEnd_AuthErrorLoginRequired(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login/end", auth.LoginEnd)
	loginEndRequest := LoginEndRequest{
		RequestURL: "https://example.com?state=foo&error=login_required",
	}
	reqBody, err := json.Marshal(&loginEndRequest)
	require.NoError(t, err)
	req, _ := http.NewRequest("POST", "/login/end", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLoginEnd_AuthErrorOther(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login/end", auth.LoginEnd)
	loginEndRequest := LoginEndRequest{
		RequestURL: "https://example.com?state=foo&error=kittens",
	}
	reqBody, err := json.Marshal(&loginEndRequest)
	require.NoError(t, err)
	req, _ := http.NewRequest("POST", "/login/end", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoginEnd_HandleLogin(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
		verifier: &FakeIDTokenVerifier{
			VerifyFn: func(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
				t := &oidc.IDToken{
					Nonce: "bar",
				}
				return t, nil
			},
		},
		oauthConfig: &FakeOauthConfig{
			ExchangeFn: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
				t := &oauth2.Token{
					AccessToken:  "floofy",
					RefreshToken: "kittens",
				}
				field := reflect.ValueOf(t).Elem().FieldByName("raw")
				setUnexportedField(field, map[string]interface{}{"id_token": "boxofkittehs"})
				return t, nil
			},
		},
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cookieStore := cookie.NewStore([]byte("secretstorage"))
	r.Use(sessions.Sessions(SessionStorage, cookieStore))
	r.POST("/login/end", auth.LoginEnd)
	loginEndRequest := LoginEndRequest{
		RequestURL: "https://example.com?state=foo&code=kittens",
	}
	reqBody, err := json.Marshal(&loginEndRequest)
	require.NoError(t, err)
	req, _ := http.NewRequest("POST", "/login/end", bytes.NewReader(reqBody))
	req.AddCookie(&http.Cookie{
		Name:  "state",
		Value: "foo",
	})
	req.AddCookie(&http.Cookie{
		Name:  "nonce",
		Value: "bar",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	data, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var response LoginEndResponse
	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	assert.Equal(t, true, response.LoggedIn)
	assert.Equal(t, true, response.Handled)

	store, err := cookieStore.Get(req, SessionStorage)
	require.NoError(t, err)

	_, ok := store.Values[IDTokenKey]
	assert.True(t, ok)

	_, ok = store.Values[TokenKey]
	assert.True(t, ok)
}

func TestLoginEnd_LoggedIn(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cookieStore := cookie.NewStore([]byte("secretstorage"))
	r.Use(sessions.Sessions(SessionStorage, cookieStore))
	r.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		t := oauth2.Token{
			AccessToken: "kittens",
		}
		session.Set(TokenKey, t)
		if err := session.Save(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Next()
	})

	r.POST("/login/end", auth.LoginEnd)
	loginEndRequest := LoginEndRequest{
		RequestURL: "https://example.com",
	}
	reqBody, err := json.Marshal(&loginEndRequest)
	require.NoError(t, err)
	req, _ := http.NewRequest("POST", "/login/end", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	data, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var response LoginEndResponse
	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	assert.True(t, response.LoggedIn)
	assert.False(t, response.Handled)
}

func TestLoginEnd_NotLoggedIn(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cookieStore := cookie.NewStore([]byte("secretstorage"))
	r.Use(sessions.Sessions(SessionStorage, cookieStore))
	r.POST("/login/end", auth.LoginEnd)
	loginEndRequest := LoginEndRequest{
		RequestURL: "https://example.com",
	}
	reqBody, err := json.Marshal(&loginEndRequest)
	require.NoError(t, err)
	req, _ := http.NewRequest("POST", "/login/end", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	data, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var response LoginEndResponse
	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	assert.False(t, response.LoggedIn)
	assert.False(t, response.Handled)
}

func TestUserInfo_NotLoggedIn(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cookieStore := cookie.NewStore([]byte("secretstorage"))
	r.Use(sessions.Sessions(SessionStorage, cookieStore))
	r.GET("/user_info", auth.UserInfo)
	req, _ := http.NewRequest("GET", "/user_info", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserInfo_LoggedIn(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
		oauthConfig: &FakeOauthConfig{
			TokenSourceFn: func(ctx context.Context, t *oauth2.Token) oauth2.TokenSource {
				return nil
			},
		},
		provider: &FakeOpenIDConnectProvider{
			UserInfoFn: func(ctx context.Context, tokenSource oauth2.TokenSource) (*oidc.UserInfo, error) {
				u := &oidc.UserInfo{
					Subject:       "8eb177be-c31e-4e2c-86fe-ab38cc7f15ad",
					Profile:       "https://example.com/profiles/testUser",
					Email:         "testuser@example.com",
					EmailVerified: false,
				}
				claims, _ := json.Marshal(map[string]interface{}{
					"sub":                "8eb177be-c31e-4e2c-86fe-ab38cc7f15ad",
					"profile":            "https://example.com/profiles/testUser",
					"email":              "testuser@example.com",
					"email_verified":     false,
					"foo":                "bar",
					"preferred_username": "testuser",
					"given_name":         "Test",
					"family_name":        "User",
					"picture":            "https://www.gravatar.com/avatar/00000000000000000000000000000000",
					"updated_at":         time.Now().Unix(),
				})
				field := reflect.ValueOf(u).Elem().FieldByName("claims")
				setUnexportedField(field, claims)
				return u, nil
			},
		},
	}
	r := gin.New()
	cookieStore := cookie.NewStore([]byte("secretstorage"))
	r.Use(sessions.Sessions(SessionStorage, cookieStore))
	r.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		t := oauth2.Token{
			AccessToken: "kittens",
		}
		session.Set(TokenKey, t)
		if err := session.Save(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Next()
	})

	r.GET("/user_info", auth.UserInfo)

	req, _ := http.NewRequest("GET", "/user_info", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	data, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var response UserInfoResponse
	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	assert.Equal(t, "testuser", response.PreferredUsername)
	assert.Equal(t, "8eb177be-c31e-4e2c-86fe-ab38cc7f15ad", response.Subject)
	assert.Equal(t, "Test", response.GivenName)
	assert.Equal(t, "User", response.FamilyName)
	assert.Equal(t, "https://www.gravatar.com/avatar/00000000000000000000000000000000", response.Picture)
}

func TestClaims(t *testing.T) {
	t.Skip("todo")
}

func TestRefresh(t *testing.T) {
	t.Skip("todo")
}

func TestLogout(t *testing.T) {
	t.Skip("todo")
}

func TestDeviceStart(t *testing.T) {
	auth := &OidcAgent{
		logger:        zap.NewExample().Sugar(),
		deviceAuthURL: "http://auth.example.com/device",
		clientID:      "cli-app",
		provider: &FakeOpenIDConnectProvider{
			EndpointFn: func() oauth2.Endpoint {
				return oauth2.Endpoint{TokenURL: "http://auth.example.com/token"}
			},
		},
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login/start", auth.DeviceStart)
	req, _ := http.NewRequest("POST", "/login/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	data, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var response DeviceStartReponse
	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	assert.Equal(t, "http://auth.example.com/device", response.DeviceAuthURL)
	assert.Equal(t, "http://auth.example.com/token", response.TokenEndpoint)
	assert.Equal(t, "cli-app", response.ClientID)
}

func setUnexportedField(field reflect.Value, value interface{}) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
		Elem().
		Set(reflect.ValueOf(value))
}
