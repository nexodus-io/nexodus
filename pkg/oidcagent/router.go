package agent

import (
	"encoding/gob"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

func init() {
	gob.Register(oauth2.Token{})
}

func NewCodeFlowRouter(auth *OidcAgent) *gin.Engine {
	r := gin.Default()
	r.Use(auth.OriginVerifier())

	store := cookie.NewStore([]byte(auth.cookieKey))
	r.Use(sessions.Sessions(SessionStorage, store))

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowCredentials = true
	corsConfig.AllowOrigins = auth.trustedOrigins
	corsConfig.ExposeHeaders = append(corsConfig.ExposeHeaders, "X-Total-Count")
	r.Use(cors.New(corsConfig))
	r.POST("/login/start", auth.LoginStart)
	r.POST("/login/end", auth.LoginEnd)
	r.GET("/user_info", auth.UserInfo)
	r.GET("/claims", auth.Claims)
	r.POST("/logout", auth.Logout)
	r.Any("/api/*proxyPath", auth.CodeFlowProxy)
	return r
}

func NewDeviceFlowRouter(auth *OidcAgent) *gin.Engine {
	r := gin.Default()
	r.POST("/login/start", auth.DeviceStart)
	r.Any("/api/*proxyPath", auth.DeviceFlowProxy)
	return r
}
