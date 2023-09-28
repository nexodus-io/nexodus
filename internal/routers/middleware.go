package routers

import (
	"context"
	_ "embed"
	"errors"
	"github.com/nexodus-io/nexodus/internal/handlers"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/nexodus-io/nexodus/internal/util/cache"
	"github.com/open-policy-agent/opa/rego"
	"golang.org/x/oauth2"
)

// key for username in gin.Context
const AuthUserName string = "_nexodus.UserName"

//go:embed token.rego
var policy string

var jwksCache = cache.NewMemoizeCache[string, string](time.Second*30, time.Second*5)

// Naive JWS Key validation
func ValidateJWT(ctx context.Context, o APIRouterOptions, jwksURI string) (func(*gin.Context), error) {
	query, err := rego.New(
		rego.Query(`result = {
			"authorized": data.token.valid_token,
			"allow": data.token.allow,
			"user_id": data.token.user_id,
			"user_name": data.token.user_name,
			"full_name": data.token.full_name,
		}`),
		rego.Store(o.Store),
		rego.Module("policy.rego", policy),
	).PrepareForEval(context.Background())
	if err != nil {
		return nil, err
	}

	return func(c *gin.Context) {
		logger := util.WithTrace(c.Request.Context(), o.Logger)

		keySet, err := jwksCache.MemoizeCanErr(jwksURI, func() (string, error) {
			return getURLAsText(ctx, jwksURI)
		})
		if err != nil {
			handlers.SendInternalServerError(c, o.Logger, err)
			c.Abort()
			return
		}

		authHeader := c.Request.Header.Get("Authorization")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		path := strings.Split(strings.TrimLeft(c.Request.URL.Path, "/"), "/")
		input := map[string]interface{}{
			"jwks":         keySet,
			"access_token": parts[1],
			"method":       c.Request.Method,
			"path":         path,
		}

		results, err := query.Eval(c.Request.Context(), rego.EvalInput(input))
		if err != nil {
			logger.Error(err)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if err != nil {
			handlers.SendInternalServerError(c, o.Logger, err)
			c.Abort()
			return
		} else if len(results) == 0 {
			handlers.SendInternalServerError(c, o.Logger, errors.New("undefined result from authz policy"))
			c.Abort()
			return
		}
		result, ok := results[0].Bindings["result"].(map[string]interface{})
		if !ok {
			handlers.SendInternalServerError(c, o.Logger.With("result", results[0].Bindings), errors.New("opa policy result is not a map"))
			c.Abort()
			return
		}
		logger = logger.With("result", result)

		authorized, ok := result["authorized"].(bool)
		if !ok {
			handlers.SendInternalServerError(c, o.Logger, errors.New("authorized is not a bool"))
			c.Abort()
			return
		}
		if !authorized {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		allowed, ok := result["allow"].(bool)
		if !ok {
			handlers.SendInternalServerError(c, o.Logger, errors.New("allow is not a bool"))
			c.Abort()
			return
		}
		if !allowed {
			logger.Debug("forbidden by authz policy")
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		userID, ok := result["user_id"].(string)
		if !ok {
			handlers.SendInternalServerError(c, o.Logger, errors.New("user_id is not a string"))
			c.Abort()
			return
		}

		username, ok := result["user_name"].(string)
		if !ok {
			handlers.SendInternalServerError(c, o.Logger, errors.New("user_name is not a string"))
			c.Abort()
			return
		}

		fullName, ok := result["full_name"].(string)
		if !ok {
			handlers.SendInternalServerError(c, o.Logger, errors.New("full_name is not a string"))
			c.Abort()
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

func getURLAsText(ctx context.Context, jwksURL string) (string, error) {

	httpClient := http.DefaultClient
	if ctx != nil {
		if ctxClient, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
			httpClient = ctxClient
		}
	}

	res, err := httpClient.Get(jwksURL)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	keySet := string(body)
	return keySet, nil
}
