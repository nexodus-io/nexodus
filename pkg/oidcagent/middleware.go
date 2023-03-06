package agent

import (
	"github.com/gin-contrib/cors"
	"github.com/go-session/session/v3"
	"github.com/redhat-et/go-oidc-agent/pkg/cookie"
	"github.com/redhat-et/go-oidc-agent/pkg/ginsession"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *OidcAgent) OriginVerifier() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		permitted := false
		for _, o := range a.trustedOrigins {
			if origin == o {
				permitted = true
				break
			}
		}
		if !permitted {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
		c.Next()
	}
}

func (auth *OidcAgent) CorsMiddleware() gin.HandlerFunc {
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowCredentials = true
	corsConfig.AllowOrigins = auth.trustedOrigins
	corsConfig.ExposeHeaders = append(corsConfig.ExposeHeaders, "X-Total-Count")
	return cors.New(corsConfig)
}

func (auth *OidcAgent) CookieSessionMiddleware() gin.HandlerFunc {
	session.InitManager(
		session.SetStore(
			cookie.NewCookieStore(
				cookie.SetHashKey([]byte(auth.cookieKey)),
			),
		),
	)
	return ginsession.New()
}
