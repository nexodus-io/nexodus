package routers

import (
	"context"
	_ "embed"
	"net/http"
	"strings"

	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/open-policy-agent/opa/rego"

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

//go:embed token.rego
var policy string

// Naive JWS Key validation
func ValidateJWT(logger *zap.SugaredLogger, jwksURI string, clientIdWeb string, clientIdCli string) (func(*gin.Context), error) {
	query, err := rego.New(
		rego.Module("policy.rego", policy),
	).PrepareForEval(context.Background())
	if err != nil {
		return nil, err
	}

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

		input := map[string]interface{}{
			"jwks":         jwksURI,
			"access_token": parts[1],
			"method":       c.Request.Method,
			"path":         c.Request.URL.Path,
		}

		results, err := query.Eval(c.Request.Context(), rego.EvalInput(input))
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if err != nil {
			logger.Error(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if len(results) == 0 {
			logger.Error("undefined result from authz policy")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if _, ok := results[0].Bindings["allow"].(bool); !ok {
			logger.Error("unexpect result type")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if !results[0].Bindings["allow"].(bool) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims := results[0].Bindings["claims"].(map[string]string)

		c.Set(gin.AuthUserKey, claims["sub"])
		if len(claims["preferred_username"]) > 0 {
			c.Set(AuthUserName, claims["preferred_username"])
		} else if len(claims["name"]) > 0 {
			c.Set(AuthUserName, claims["name"])
		} else {
			logger.Debugf("Not able to determine a name for this user -- %s", claims["sub"])
		}
		// c.Set(AuthUserScope, claims.Scope)
		logger.Debugf("user-id is %s", claims["sub"])
		c.Next()
	}, nil
}
