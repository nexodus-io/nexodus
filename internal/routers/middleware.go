package routers

import (
	"net/http"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// key for username in gin.Context
const AuthUserName string = "_apex.UserName"

type Claims struct {
	Scope      string `json:"scope"`
	FullName   string `json:"name"`
	UserName   string `json:"preferred_username"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Subject    string `json:"sub"`
}

// Naive JWS Key validation
func ValidateJWT(verifier *oidc.IDTokenVerifier) func(*gin.Context) {
	return func(c *gin.Context) {
		log.Debug("validate jwt")
		authz := c.Request.Header.Get("Authorization")
		if authz == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authz, " ")
		if len(parts) != 2 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		log.Debug("verifying token")
		token, err := verifier.Verify(c.Request.Context(), parts[1])
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		log.Debug("getting claims")
		var claims Claims
		if err := token.Claims(&claims); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		log.Debugf("claims: %+v", claims)
		c.Set(gin.AuthUserKey, claims.Subject)
		c.Set(AuthUserName, claims.UserName)
		// c.Set(AuthUserScope, claims.Scope)
		log.Debugf("user-id is %s", claims.Subject)
		c.Next()
	}
}
