package routers

import (
	"github.com/nexodus-io/nexodus/internal/util"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// key for username in gin.Context
const AuthUserName string = "_nexodus.UserName"

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
		logger := util.WithTrace(c.Request.Context(), logger)
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
		token, err := verifier.Verify(c.Request.Context(), parts[1])
		if err != nil {
			logger.With("error", err).Debug("verification failed")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Skip this for now.
		// TODO: Add more robust aud/azp checks
		// Dex sets aud... not sure about azp
		// Keycloak's aud is account in an access token, but azp is the client-id
		// for _, audience := range token.Audience {
		//	if audience != clientIdWeb && audience != clientIdCli {
		//		c.AbortWithStatus(http.StatusUnauthorized)
		//		return
		//	}
		// }

		logger.Debug("getting claims")
		var claims Claims
		if err := token.Claims(&claims); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		logger.Debugf("claims: %+v", claims)
		c.Set(gin.AuthUserKey, claims.Subject)
		if len(claims.UserName) > 0 {
			c.Set(AuthUserName, claims.UserName)
		} else if len(claims.FullName) > 0 {
			c.Set(AuthUserName, claims.FullName)
		} else {
			logger.Debugf("Not able to determine a name for this user -- %s", claims.Subject)
		}
		// c.Set(AuthUserScope, claims.Scope)
		logger.Debugf("user-id is %s", claims.Subject)
		c.Next()
	}
}
