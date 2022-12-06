package routers

import (
	"net/http"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
func ValidateJWT(logger *zap.SugaredLogger, verifier *oidc.IDTokenVerifier, clientIdWeb string, clientIdCli string) func(*gin.Context) {
	return func(c *gin.Context) {
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

		logger.Debug("verifying token")
		token, err := verifier.Verify(c.Request.Context(), parts[1])
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		for _, audience := range token.Audience {
			if audience != clientIdWeb && audience != clientIdCli {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
		}

		logger.Debug("getting claims")
		var claims Claims
		if err := token.Claims(&claims); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		logger.Debugf("claims: %+v", claims)
		c.Set(gin.AuthUserKey, claims.Subject)
		c.Set(AuthUserName, claims.UserName)
		// c.Set(AuthUserScope, claims.Scope)
		logger.Debugf("user-id is %s", claims.Subject)
		c.Next()
	}
}
