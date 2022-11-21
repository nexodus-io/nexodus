package agent

import (
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
