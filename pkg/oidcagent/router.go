package agent

import "github.com/gin-gonic/gin"

func NewRouter(auth *OidcAgent) *gin.Engine {
	r := gin.Default()
	r.Use(auth.OriginVerifier())
	r.POST("/login/start", auth.LoginStart)
	r.POST("/login/end", auth.LoginEnd)
	r.GET("/user_info", auth.UserInfo)
	r.GET("/claims", auth.Claims)
	r.GET("/logout", auth.Logout)
	r.Any("/api/*any", auth.Proxy)
	return r
}
