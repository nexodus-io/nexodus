package oidcagent

import (
	"encoding/gob"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

func init() {
	gob.Register(oauth2.Token{})
}

func NewCodeFlowRouter(auth *OidcAgent) *gin.Engine {
	r := gin.Default()
	r.Use(auth.CorsMiddleware())
	r.Use(auth.CookieSessionMiddleware())
	AddCodeFlowRoutes(r, auth)
	r.Any("/api/*proxyPath", auth.CodeFlowProxy)
	return r
}

func AddCodeFlowRoutes(r gin.IRouter, auth *OidcAgent) {
	r.Use(auth.OriginVerifier())
	r.POST("/login/start", auth.LoginStart)
	r.POST("/login/end", auth.LoginEnd)
	r.GET("/user_info", auth.UserInfo)
	r.GET("/claims", auth.Claims)
	r.POST("/logout", auth.Logout)
	// r.GET("/check_auth", auth.CheckAuth)
	r.POST("/refresh", auth.Refresh)
}

func NewDeviceFlowRouter(auth *OidcAgent) *gin.Engine {
	r := gin.Default()
	AddDeviceFlowRoutes(r, auth)
	r.Any("/api/*proxyPath", auth.DeviceFlowProxy)
	return r
}

func AddDeviceFlowRoutes(r gin.IRouter, auth *OidcAgent) {
	r.POST("/login/start", auth.DeviceStart)
}
