package oidcagent

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/pkg/ginsession"
	"github.com/nexodus-io/nexodus/pkg/oidcagent/models"
	"golang.org/x/oauth2"
)

const (
	TokenKey   = "token"
	IDTokenKey = "id_token"
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
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec: G402
		}
		client := &http.Client{Transport: transport}
		return oidc.ClientContext(parent, client)
	}
	return c.Request.Context()
}

// LoginStart initiates the OIDC login process.
// @Summary      Initiates OIDC Web Login
// @Description  Generates state and nonce, then redirects the user to the OAuth2 authorization URL.
// @Id           WebStart
// @Tags         Auth
// @Accepts      json
// @Produce      json
// @Param		 redirect     query  string   true "URL to redirect to if login succeeds"
// @Param		 failure     query  string   false "URL to redirect to if login fails (optional)"
// @Success      302 {string} string "Redirects to the OAuth2 authorization URL"
// @Router       /web/login/start [get]
func (o *OidcAgent) LoginStart(c *gin.Context) {
	logger := o.logger
	logger.Debug("handling login start request")

	query := struct {
		Redirect string `form:"redirect"`
		Failure  string `form:"failure"`
	}{}
	err := c.ShouldBindQuery(&query)
	if err != nil {
		logger.With("error", err).Info("unable to bind query")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if query.Redirect == "" {
		logger.Info("redirect URL missing")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	state, err := randString(16)
	if err != nil {
		logger.With("error", err).Info("unable generate random state")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	nonce, err := randString(16)
	if err != nil {
		logger.With("error", err).Info("unable generate random nonce")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("redirect", query.Redirect, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	c.SetCookie("failure", query.Failure, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	c.SetCookie("state", state, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	c.SetCookie("nonce", nonce, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	url := o.oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce))
	c.Redirect(http.StatusFound, url)
}

// LoginEnd completes the OIDC login process.
// @Summary      Completes OIDC Web Login
// @Description  Handles the callback from the OAuth2/OpenID provider and verifies the tokens.
// @Id           WebEnd
// @Tags         Auth
// @Accepts      json
// @Produce      json
// @Param		 code     query  string   true "oauth2 code from authorization server"
// @Param		 state    query  string   true "state value from the login start request"
// @Param		 error    query  string   true "error message if login failed"
// @Success      302 {string} string "Redirects to the URLs specified in the login start request"
// @Router       /web/login/end [get]
func (o *OidcAgent) LoginEnd(c *gin.Context) {

	logger := o.logger
	ctx := o.prepareContext(c)
	logger.Debug("handling login end request")

	query := struct {
		Code  string `form:"code"`
		State string `form:"state"`
		Error string `form:"error"`
	}{}
	err := c.ShouldBindQuery(&query)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	redirectURL, err := c.Cookie("redirect")
	if err != nil {
		logger.With("error", err).Info("unable to access redirect cookie")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if redirectURL == "" {
		logger.Info("redirect URL missing")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	c.SetCookie("redirect", "", -1, "/", "", c.Request.URL.Scheme == "https", true)
	failureURL, _ := c.Cookie("failure")
	c.SetCookie("failure", "", -1, "/", "", c.Request.URL.Scheme == "https", true)
	if failureURL == "" {
		failureURL = redirectURL
	}

	failed := query.State != "" && query.Error != ""
	if failed {
		logger.Debug("login failed")
		c.Redirect(302, failureURL)
		return
	}

	if query.State == "" || query.Code == "" {
		logger.Debug("state or code missing")
		c.Redirect(302, failureURL)
		return
	}

	originalState, err := c.Cookie("state")
	if err != nil {
		logger.With("error", err).Debug("unable to access state cookie")
		c.Redirect(302, failureURL)
		return
	}

	c.SetCookie("state", "", -1, "/", "", c.Request.URL.Scheme == "https", true)
	if query.State != originalState {
		logger.With("error", err).Debug("state does not match")
		c.Redirect(302, failureURL)
		return
	}

	session := ginsession.FromContext(c)

	if query.Code == "logout" {
		session.Delete(IDTokenKey)
		session.Delete(TokenKey)
		if err := session.Save(); err != nil {
			c.Redirect(302, failureURL)
			return
		}
		c.Redirect(302, redirectURL)
		return
	}

	nonce, err := c.Cookie("nonce")
	if err != nil {
		logger.With("error", err).Debug("unable to get nonce cookie")
		c.Redirect(302, failureURL)
		return
	}
	c.SetCookie("nonce", "", -1, "/", "", c.Request.URL.Scheme == "https", true)

	oauth2Token, err := o.oauthConfig.Exchange(ctx, query.Code)
	if err != nil {
		logger.With("error", err).Debug("unable to exchange token")
		c.Redirect(302, failureURL)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		logger.With("ok", ok).Debug("unable to get id_token")
		c.Redirect(302, failureURL)
		return
	}

	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		logger.With("error", err).Debug("unable to verify id_token")
		c.Redirect(302, failureURL)
		return
	}

	if idToken.Nonce != nonce {
		logger.Debug("nonce does not match")
		c.Redirect(302, failureURL)
		return
	}

	tokenString, err := tokenToJSONString(oauth2Token)
	if err != nil {
		logger.Debug("can't convert token to string")
		c.Redirect(302, failureURL)
		return
	}
	session.Set(TokenKey, tokenString)
	session.Set(IDTokenKey, rawIDToken)
	if err := session.Save(); err != nil {
		logger.With("error", err, "id_token_size", len(rawIDToken)).Debug("can't save session storage")
		c.Redirect(302, failureURL)
		return
	}

	logger.With("session_id", session.SessionID()).Debug("user is logged in")
	c.Redirect(http.StatusFound, redirectURL)
}

// UserInfo retrieves details about the currently authenticated user.
// @Summary     Retrieve Current User Information
// @Description Fetches and returns information for the user who is currently authenticated.
// @Id          UserInfo
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Success     200 {object} models.UserInfoResponse
// @Router      /web/user_info [get]
func (o *OidcAgent) UserInfo(c *gin.Context) {
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token, err := JsonStringToToken(tokenRaw.(string))
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
		EmailVerified bool   `json:"email_verified"`
		Email         string `json:"email"`
		Username      string `json:"preferred_username"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
		UpdatedAt     int64  `json:"updated_at"`
	}

	err = info.Claims(&claims)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	o.logger.With("claims", claims).Debug("got claims from id_token")
	res := models.UserInfoResponse{
		Subject:           info.Subject,
		PreferredUsername: claims.Username,
		GivenName:         claims.GivenName,
		UpdatedAt:         int64(claims.UpdatedAt),
		FamilyName:        claims.FamilyName,
		Picture:           claims.Picture,
		EmailVerified:     claims.EmailVerified,
		Email:             claims.Email,
	}

	c.JSON(http.StatusOK, res)
}

// Claims fetches the claims associated with the user's access token.
// @Summary     Get Access Token Claims
// @Description Retrieves the claims present in the user's access token.
// @Id          Claims
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Success     200 {object} map[string]interface{}
// @Router      /web/claims [get]
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

// Refresh updates the user's access token.
// @Summary     Refresh Access Token
// @Description Obtains and updates a new access token for the user.
// @Id          Refresh
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Success     204
// @Router      /web/refresh [post]
func (o *OidcAgent) Refresh(c *gin.Context) {
	logger := o.logger

	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	tokenRaw, ok := session.Get(TokenKey)

	if !ok {
		logger.Debug("No existing token in session, unauthorized")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token, err := JsonStringToToken(tokenRaw.(string))
	if err != nil {
		logger.Debug("Failed to convert token from JSON string %v", tokenRaw)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	src := o.oauthConfig.TokenSource(ctx, token)
	newToken, err := src.Token()

	if err != nil {
		logger.Debug("Failed to refresh token: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	tokenString, err := tokenToJSONString(newToken)
	if err != nil {
		logger.Debug("Failed to convert new token to string: %v", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	session.Set(TokenKey, tokenString)
	if err := session.Save(); err != nil {
		logger.Debug("Failed to save new token in session: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

// Logout provides the URL to log out the current user.
// @Summary     Generate Logout URL
// @Description Provides the URL to initiate the logout process for the current user.
// @Id          Logout
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param		redirect     query  string   true "URL to redirect to after logout"
// @Success     302 {string} string "Redirects to the OAuth2 logout URL"
// @Router      /web/logout [get]
func (o *OidcAgent) Logout(c *gin.Context) {
	logger := o.logger

	query := struct {
		Redirect string `form:"redirect"`
	}{}
	err := c.ShouldBindQuery(&query)
	if err != nil {
		logger.With("error", err).Info("unable to bind query")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if query.Redirect == "" {
		logger.Info("redirect URL missing")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	session := ginsession.FromContext(c)
	idToken, ok := session.Get(IDTokenKey)
	if !ok {
		// If the user is not logged in, redirect to the specified URL
		c.Redirect(http.StatusFound, query.Redirect)
		return
	}
	state, err := randString(16)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Yeah we reuse the LoginEnd function here to complete the logout process
	u, err := url.Parse(o.redirectURL)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	q := u.Query()
	q.Set("code", "logout")
	q.Set("state", state)
	u.RawQuery = q.Encode()

	// Redirect to the OIDC provider's logout URL
	logoutURL, err := o.LogoutURL(idToken.(string), u.String())
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("redirect", query.Redirect, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	c.SetCookie("state", state, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	c.Redirect(http.StatusFound, logoutURL.String())
}

func (o *OidcAgent) CodeFlowProxy(c *gin.Context) {
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token, err := JsonStringToToken(tokenRaw.(string))
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

// CheckAuth checks if the user is authenticated.
// @Summary     Check Authentication
// @Description Checks if the user is currently authenticated
// @Id          CheckAuth
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Success     200 {object} map[string]bool "logged_in status will be returned"
// @Router      /check/auth [get]
func (o *OidcAgent) CheckAuth(c *gin.Context) {
	loggedIn := isLoggedIn(c)
	c.JSON(http.StatusOK, gin.H{"logged_in": loggedIn})
}

// DeviceStart initiates the device login process.
// @Summary     Start Login
// @Description Starts a device login request
// @Id          DeviceStart
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Success     200 {object} models.DeviceStartResponse
// @Router      /device/login/start [post]
func (o *OidcAgent) DeviceStart(c *gin.Context) {
	now := time.Now()
	c.JSON(http.StatusOK, models.DeviceStartResponse{
		DeviceAuthURL: o.deviceAuthURL,
		Issuer:        o.oidcIssuer,
		ClientID:      o.clientID,
		ServerTime:    &now,
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

func JsonStringToToken(s string) (*oauth2.Token, error) {
	var t oauth2.Token
	if err := json.Unmarshal([]byte(s), &t); err != nil {
		return nil, err
	}
	return &t, nil

}
