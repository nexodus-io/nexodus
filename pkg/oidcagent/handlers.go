package oidcagent

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/pkg/ginsession"
	"golang.org/x/oauth2"
)

const (
	SessionStorage = "apex_session"
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

func (o *OidcAgent) prepareContext(c *gin.Context) context.Context {
	if o.insecureTLS {
		parent := c.Request.Context()
		// #nosec: G402
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: transport}
		return oidc.ClientContext(parent, client)
	}
	return c.Request.Context()
}

// LoginStart starts a login request
// @Summary      Start Login
// @Description  Starts a login request for the frontend application
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  LoginStartResponse
// @Router       /login/start [post]
func (o *OidcAgent) LoginStart(c *gin.Context) {
	logger := o.logger
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

	logger = logger.With(
		"state", state,
		"nonce", nonce,
	)

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("state", state, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	c.SetCookie("nonce", nonce, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	logger.Debug("set cookies")
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

	logger := o.logger
	ctx := o.prepareContext(c)
	logger.Debug("handling login end request")

	values := requestURL.Query()
	code := values.Get("code")
	state := values.Get("state")
	queryErr := values.Get("error")

	failed := state != "" && queryErr != ""

	if failed {
		logger.Debug("login failed")
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
		logger.Debug("login success")
		originalState, err := c.Cookie("state")
		if err != nil {
			logger.With(
				"error", err,
			).Debug("unable to access state cookie")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.SetCookie("state", "", -1, "/", "", c.Request.URL.Scheme == "https", true)
		if state != originalState {
			logger.With(
				"error", err,
			).Debug("state does not match")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		nonce, err := c.Cookie("nonce")
		if err != nil {
			logger.With(
				"error", err,
			).Debug("unable to get nonce cookie")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.SetCookie("nonce", "", -1, "/", "", c.Request.URL.Scheme == "https", true)

		oauth2Token, err := o.oauthConfig.Exchange(ctx, code)
		if err != nil {
			logger.With(
				"error", err,
			).Debug("unable to exchange token")
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			logger.With(
				"ok", ok,
			).Debug("unable to get id_token")
			_ = c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("no id_token field in oauth2 token"))
			return
		}

		idToken, err := o.verifier.Verify(ctx, rawIDToken)
		if err != nil {
			logger.With(
				"error", err,
			).Debug("unable to verify id_token")
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if idToken.Nonce != nonce {
			logger.Debug("nonce does not match")
			_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("nonce did not match"))
			return
		}

		session := ginsession.FromContext(c)
		tokenString, err := tokenToJSONString(oauth2Token)
		if err != nil {
			logger.Debug("can't convert token to string")
			_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("can't convert token to string"))
			return
		}
		session.Set(TokenKey, tokenString)
		session.Set(IDTokenKey, rawIDToken)
		if err := session.Save(); err != nil {
			logger.With("error", err,
				"id_token_size", len(rawIDToken)).Debug("can't save session storage")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		logger.With("session_id", session.SessionID()).Debug("user is logged in")
		loggedIn = true
	} else {
		logger.Debug("checking if user is logged in")
		loggedIn = isLoggedIn(c)
	}

	session := ginsession.FromContext(c)
	logger.With("session_id", session.SessionID()).With("logged_in", loggedIn).Debug("complete")
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
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token, err := jsonStringToToken(tokenRaw.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	src := o.oauthConfig.TokenSource(ctx, token)

	info, err := o.provider.UserInfo(ctx, src)
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
	o.logger.With("claims", claims).Debug("got claims from id_token")
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
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	idTokenRaw, ok := session.Get(IDTokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	idToken, err := o.verifier.Verify(ctx, idTokenRaw.(string))
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
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token, err := jsonStringToToken(tokenRaw.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	src := o.oauthConfig.TokenSource(ctx, token)
	newToken, err := src.Token()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tokenString, err := tokenToJSONString(newToken)
	if err != nil {
		o.logger.Debug("can't convert token to string")
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("can't convert token to string"))
		return
	}
	session.Set(TokenKey, tokenString)
	if err := session.Save(); err != nil {
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
	session := ginsession.FromContext(c)
	idToken, ok := session.Get(IDTokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	session.Delete(IDTokenKey)
	session.Delete(TokenKey)
	if err := session.Save(); err != nil {
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

func (o *OidcAgent) CodeFlowProxy(c *gin.Context) {
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token, err := jsonStringToToken(tokenRaw.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	// Use a static token source to avoid automatically
	// refreshing the token - this needs to be handled
	// by the frontend
	src := oauth2.StaticTokenSource(token)
	client := oauth2.NewClient(ctx, src)
	proxy := httputil.NewSingleHostReverseProxy(o.backend)

	// Use the client transport
	proxy.Transport = client.Transport
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = o.backend.Host
		req.URL.Scheme = o.backend.Scheme
		req.URL.Host = o.backend.Host
		req.URL.Path = c.Param("proxyPath")
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func isLoggedIn(c *gin.Context) bool {
	session := ginsession.FromContext(c)
	_, ok := session.Get(TokenKey)
	return ok
}

func (o *OidcAgent) DeviceStart(c *gin.Context) {
	c.JSON(http.StatusOK, DeviceStartReponse{
		DeviceAuthURL: o.deviceAuthURL,
		Issuer:        o.oidcIssuer,
		ClientID:      o.clientID,
	})
}

func (o *OidcAgent) DeviceFlowProxy(c *gin.Context) {
	proxy := httputil.NewSingleHostReverseProxy(o.backend)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = o.backend.Host
		req.URL.Scheme = o.backend.Scheme
		req.URL.Host = o.backend.Host
		req.URL.Path = c.Param("proxyPath")
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func tokenToJSONString(t *oauth2.Token) (string, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func jsonStringToToken(s string) (*oauth2.Token, error) {
	var t oauth2.Token
	if err := json.Unmarshal([]byte(s), &t); err != nil {
		return nil, err
	}
	return &t, nil

}
