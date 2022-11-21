package agent

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	"github.com/go-session/session"
	"golang.org/x/oauth2"
)

const (
	SessionStorage = "session"
	TokenKey       = "token"
	IDTokenKey     = "id_token"
)

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// LoginStart starts a login request
// @Summary      Start Login
// @Description  Starts a login request for the frontend application
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  LoginStartResponse
// @Router       /login/start [post]
func (o *OidcAgent) LoginStart(c *gin.Context) {
	state, err := randString(16)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	nonce, err := randString(16)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.SetCookie("state", state, int(time.Hour.Seconds()), "/", o.domain, false, true)
	c.SetCookie("nonce", nonce, int(time.Hour.Seconds()), "/", o.domain, false, true)
	c.JSON(http.StatusOK, LoginStartReponse{
		AuthorizationRequestURL: o.oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce)),
	})
}

// LoginEnd ends a login request
// @Summary      End Login
// @Description  Called by the frontend to finish the auth flow or check whether the user is logged in
// @Accepts		 json
// @Produce      json
// @Param        data  body   LoginEndRequest  true "End Login"
// @Success      200
// @Router       /login/end [post]
func (o *OidcAgent) LoginEnd(c *gin.Context) {
	var data LoginEndRequest
	err := c.BindJSON(&data)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	requestURL, err := url.Parse(data.RequestURL)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	values := requestURL.Query()
	code := values.Get("code")
	state := values.Get("state")
	queryErr := values.Get("error")

	failed := state != "" && queryErr != ""

	if failed {
		var status int
		if queryErr == "login_required" {
			status = http.StatusUnauthorized
		} else {
			status = http.StatusBadRequest
		}
		c.AbortWithStatus(status)
		return
	}

	handleAuth := state != "" && code != ""

	loggedIn := false
	if handleAuth {
		originalState, err := c.Cookie("state")
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.SetCookie("state", "", -1, "/", o.domain, false, true)
		if state != originalState {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		nonce, err := c.Cookie("nonce")
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.SetCookie("nonce", "", -1, "/", o.domain, false, true)

		oauth2Token, err := o.oauthConfig.Exchange(c.Request.Context(), code)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			_ = c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("no id_token field in oauth2 token"))
			return
		}

		idToken, err := o.verifier.Verify(c.Request.Context(), rawIDToken)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if idToken.Nonce != nonce {
			_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("nonce did not match"))
			return
		}

		store, err := createSessionStorage(c)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		store.Set(TokenKey, *oauth2Token)
		store.Set(IDTokenKey, rawIDToken)
		if err := store.Save(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		loggedIn = true
	} else {
		loggedIn = isLoggedIn(c)
	}

	res := LoginEndResponse{
		Handled:  handleAuth,
		LoggedIn: loggedIn,
	}
	c.JSON(http.StatusOK, res)
}

// UserInfo gets information about the current user
// @Summary      User Info
// @Description  Returns information about the currently logged-in user
// @Accepts		 json
// @Produce      json
// @Success      200	{object}	UserInfo
// @Router       /user_info [get]
func (o *OidcAgent) UserInfo(c *gin.Context) {
	store, err := getSessionStorage(c)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	tokenRaw, ok := store.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	token, ok := tokenRaw.(oauth2.Token)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	src := o.oauthConfig.TokenSource(c.Request.Context(), &token)

	info, err := o.provider.UserInfo(c.Request.Context(), src)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var claims struct {
		Username   string `json:"preferred_username"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Picture    string `json:"picture"`
		UpdatedAt  int64  `json:"updated_at"`
	}

	err = info.Claims(&claims)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	res := UserInfoResponse{
		Subject:           info.Subject,
		PreferredUsername: claims.Username,
		GivenName:         claims.GivenName,
		UpdatedAt:         int64(claims.UpdatedAt),
		FamilyName:        claims.FamilyName,
		Picture:           claims.Picture,
	}

	c.JSON(http.StatusOK, res)
}

// Claims gets the claims of the users access token
// @Summary      Claims
// @Description  Gets the claims of the users access token
// @Accepts		 json
// @Produce      json
// @Success      200	{object}	map[string]interface{}
// @Router       /claims [get]
func (o *OidcAgent) Claims(c *gin.Context) {
	store, err := getSessionStorage(c)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	idTokenRaw, ok := store.Get(IDTokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	idToken, err := o.verifier.Verify(c.Request.Context(), idTokenRaw.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var claims map[string]interface{}
	err = idToken.Claims(claims)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, claims)
}

// Refresh refreshes the access token
// @Summary      Refresh
// @Description  Refreshes the access token
// @Accepts		 json
// @Produce      json
// @Success      200	{object}	Claims
// @Router       /refresh [get]
func (o *OidcAgent) Refresh(c *gin.Context) {
	store, err := getSessionStorage(c)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	tokenRaw, ok := store.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	token, ok := tokenRaw.(oauth2.Token)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	src := o.oauthConfig.TokenSource(c.Request.Context(), &token)
	newToken, err := src.Token()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	store.Set(TokenKey, *newToken)
	if err := store.Save(); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusNoContent)
}

// Logout returns the URL to logout the current user
// @Summary      Logout
// @Description  Returns the URL to logout the current user
// @Accepts		 json
// @Produce      json
// @Success      200	{object}	LogoutResponse
// @Router       /logout [post]
func (o *OidcAgent) Logout(c *gin.Context) {
	store, err := getSessionStorage(c)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	idToken, ok := store.Get(IDTokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	err = session.Destroy(c.Request.Context(), c.Writer, c.Request)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	logoutURL, err := o.LogoutURL(idToken.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, LogoutResponse{
		LogoutURL: logoutURL.String(),
	})
}

func (o *OidcAgent) Proxy(c *gin.Context) {
	store, err := getSessionStorage(c)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	tokenRaw, ok := store.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	token, ok := tokenRaw.(oauth2.Token)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Header("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	path := strings.TrimPrefix(c.Request.URL.Path, "/api")
	dest := o.backend.JoinPath(path)
	c.Redirect(http.StatusFound, dest.String())
}

func createSessionStorage(c *gin.Context) (session.Store, error) {
	store, err := session.Start(c.Request.Context(), c.Writer, c.Request)
	if err != nil {
		return nil, err
	}
	c.Set(SessionStorage, store)
	return store, nil
}

func getSessionStorage(c *gin.Context) (session.Store, error) {
	storeRaw, ok := c.Get(SessionStorage)
	if !ok {
		return nil, fmt.Errorf("no session storage found")
	}
	store, ok := storeRaw.(session.Store)
	if !ok {
		return nil, fmt.Errorf("no session storage found")
	}
	return store, nil
}

func isLoggedIn(c *gin.Context) bool {
	store, err := getSessionStorage(c)
	if err != nil {
		return false
	}
	_, ok := store.Get(TokenKey)
	return ok
}
