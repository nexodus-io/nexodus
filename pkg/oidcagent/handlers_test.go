package oidcagent

import (
	"context"
	"encoding/json"
	"github.com/nexodus-io/nexodus/pkg/oidcagent/models"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/go-session/session/v3"
	"github.com/nexodus-io/nexodus/pkg/cookie"
	"github.com/nexodus-io/nexodus/pkg/ginsession"
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
	r.GET("/login/start", auth.LoginStart)
	req, _ := http.NewRequest("GET", "/login/start?redirect=a&failure=b", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://auth.example.com/auth?state=foo&client_id=bar", w.Header().Get("Location"))

	cookies := w.Result().Cookies()
	assert.Equal(t, 4, len(cookies))
	assert.Equal(t, "redirect", cookies[0].Name)
	assert.Equal(t, "failure", cookies[1].Name)
	assert.Equal(t, "state", cookies[2].Name)
	assert.Equal(t, "nonce", cookies[3].Name)
}

func TestLoginEnd_AuthErrorOther(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/login/end", auth.LoginEnd)
	req, _ := http.NewRequest("GET", "/login/end", nil)
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
				setUnexportedField(field, map[string]interface{}{
					"id_token":      "boxofkittehs",
					"access_token":  "floofy",
					"refresh_token": "kittens",
				})
				return t, nil
			},
		},
	}

	session.InitManager(
		session.SetStore(
			cookie.NewCookieStore(
				cookie.SetHashKey([]byte("secretkey")),
			),
		),
	)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginsession.New())
	r.GET("/login/end", auth.LoginEnd)

	req, _ := http.NewRequest("GET", "/login/end?state=foo&code=kittens", nil)
	req.AddCookie(&http.Cookie{
		Name:  "redirect",
		Value: "https://example.com/ok",
	})
	req.AddCookie(&http.Cookie{
		Name:  "failure",
		Value: "https://example.com/failed",
	})
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

	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://example.com/ok", w.Header().Get("Location"))

}

func TestLoginEnd_NotLoggedIn(t *testing.T) {
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
				setUnexportedField(field, map[string]interface{}{
					"id_token":      "boxofkittehs",
					"access_token":  "floofy",
					"refresh_token": "kittens",
				})
				return t, nil
			},
		},
	}

	session.InitManager(
		session.SetStore(
			cookie.NewCookieStore(
				cookie.SetHashKey([]byte("secretkey")),
			),
		),
	)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginsession.New())
	r.GET("/login/end", auth.LoginEnd)

	req, _ := http.NewRequest("GET", "/login/end?state=foo&code=kittens&error=failed", nil)
	req.AddCookie(&http.Cookie{
		Name:  "redirect",
		Value: "https://example.com/ok",
	})
	req.AddCookie(&http.Cookie{
		Name:  "failure",
		Value: "https://example.com/failed",
	})
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

	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://example.com/failed", w.Header().Get("Location"))
}

func TestUserInfo_NotLoggedIn(t *testing.T) {
	auth := &OidcAgent{
		logger: zap.NewExample().Sugar(),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	session.InitManager(
		session.SetStore(
			cookie.NewCookieStore(
				cookie.SetCookieName("demo_cookie_store_id"),
				cookie.SetHashKey([]byte(auth.cookieKey)),
			),
		),
	)
	r.Use(ginsession.New())
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
	session.InitManager(
		session.SetStore(
			cookie.NewCookieStore(
				cookie.SetCookieName("demo_cookie_store_id"),
				cookie.SetHashKey([]byte(auth.cookieKey)),
			),
		),
	)
	r.Use(ginsession.New())
	r.Use(func(c *gin.Context) {
		session := ginsession.FromContext(c)
		t := oauth2.Token{
			AccessToken: "kittens",
		}
		token, _ := tokenToJSONString(&t)
		session.Set(TokenKey, token)
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

	var response models.UserInfoResponse
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
		oidcIssuer:    "http://auth.example.com",
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

	var response models.DeviceStartResponse
	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	assert.Equal(t, "http://auth.example.com/device", response.DeviceAuthURL)
	assert.Equal(t, "http://auth.example.com", response.Issuer)
	assert.Equal(t, "cli-app", response.ClientID)
}

func setUnexportedField(field reflect.Value, value interface{}) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
		Elem().
		Set(reflect.ValueOf(value))
}
