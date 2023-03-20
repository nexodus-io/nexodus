package routers

import (
	"context"
	_ "embed"
	"io"
	"net/http"
	"strings"

	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/open-policy-agent/opa/rego"
	"golang.org/x/oauth2"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// key for username in gin.Context
const AuthUserName string = "_nexodus.UserName"

//go:embed token.rego
var policy string

// Naive JWS Key validation
func ValidateJWT(ctx context.Context, logger *zap.SugaredLogger, jwksURI string, clientIdWeb string, clientIdCli string) (func(*gin.Context), error) {
	query, err := rego.New(
		rego.Query(`result = {
			"authorized": data.token.valid_token,
			"allow": data.token.allow,
			"user_id": data.token.user_id,
			"user_name": data.token.user_name,
			"full_name": data.token.full_name,
		}`),
		rego.Module("policy.rego", policy),
	).PrepareForEval(context.Background())
	if err != nil {
		return nil, err
	}

	var httpClient *http.Client
	if ctx != nil {
		if ctxClient, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
			httpClient = ctxClient
		}
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	res, err := httpClient.Get(jwksURI)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	keySet := string(body)

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
			"jwks":         keySet,
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
		}
		result, ok := results[0].Bindings["result"].(map[string]interface{})
		if !ok {
			logger.With("result", results[0].Bindings).Error("opa policy result is not a map")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		logger = logger.With("result", result)

		authorized, ok := result["authorized"].(bool)
		if !ok {
			logger.Error("authorized is not a bool")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if !authorized {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		allowed, ok := result["allow"].(bool)
		if !ok {
			logger.Error("allow is not a bool")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if !allowed {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		userID, ok := result["user_id"].(string)
		if !ok {
			logger.Error("user_id is not a string")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		username, ok := result["user_name"].(string)
		if !ok {
			logger.Error("user_name is not a string")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		fullName, ok := result["full_name"].(string)
		if !ok {
			logger.Error("full_name is not a string")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Set(gin.AuthUserKey, userID)
		if len(username) > 0 {
			c.Set(AuthUserName, username)
		} else if len(fullName) > 0 {
			c.Set(AuthUserName, fullName)
		} else {
			logger.Debugf("Not able to determine a name for this user -- %s", userID)
		}

		logger.Debugf("user-id is %s", userID)
		c.Next()
	}, nil
}
