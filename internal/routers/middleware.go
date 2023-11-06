package routers

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/handlers"
	"github.com/redis/go-redis/v9"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	csmap "github.com/mhmtszr/concurrent-swiss-map"
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
func ValidateJWT(ctx context.Context, o APIRouterOptions, jwksURI string, nexodusJWKS string) (func(*gin.Context), error) {
	query, err := rego.New(
		rego.Query(`result = {
			"authorized": data.token.valid_token,
			"allow": data.token.allow,
			"user_id": data.token.user_id,
			"user_name": data.token.user_name,
			"full_name": data.token.full_name,
			"token_payload": data.token.token_payload,
		}`),
		rego.Store(o.Store),
		rego.Module("policy.rego", policy),
	).PrepareForEval(context.Background())
	if err != nil {
		return nil, err
	}

	userLimiters := csmap.Create[string, *UserLimiters](
		// set the number of map shards to scale with CPUs allocated to the apiserver:
		csmap.WithShardCount[string, *UserLimiters](2*uint64(runtime.GOMAXPROCS(-1))),
		// set the total capacity, every shard map has total capacity/shard count capacity. the default value is 0.
		csmap.WithSize[string, *UserLimiters](1000),
	)

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
			"nexodus_jwks": nexodusJWKS,
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

		idpUserID, ok := result["user_id"].(string)
		if !ok {
			handlers.SendInternalServerError(c, o.Logger, errors.New("user_id is not a string"))
			c.Abort()
			return
		}

		idpUserName, ok := result["user_name"].(string)
		if !ok {
			handlers.SendInternalServerError(c, o.Logger, errors.New("user_name is not a string"))
			c.Abort()
			return
		}

		idpFullName, ok := result["full_name"].(string)
		if !ok {
			handlers.SendInternalServerError(c, o.Logger, errors.New("full_name is not a string"))
			c.Abort()
			return
		}

		claims := result["token_payload"].(map[string]interface{})
		c.Set("_nexodus.Claims", claims)

		if len(idpUserName) == 0 {
			idpUserName = idpFullName
		}

		var limiters *UserLimiters
		userLimiters.SetIf(idpUserID, func(value *UserLimiters, found bool) (*UserLimiters, bool) {
			if !found {
				value = NewUserLimiters()
			}
			value.Add()
			limiters = value
			return value, !found
		})
		defer func() {
			userLimiters.DeleteIf(idpUserID, func(value *UserLimiters) bool {
				return value.Done() == 0
			})
		}()

		prefixId := fmt.Sprintf("%s:%s", handlers.CachePrefix, idpUserID)
		cachedUserId := ""

		// for now just use the concurrency limiter to serialize the cache lookup and create user if not exists
		// in the future we can use the concurrency limiter to limit the number of concurrent requests to  other
		//  requests in the apiserver.  We may need different concurrency levels for things like device requests.
		// NOTE: This does not prevent concurrent access across the system. It is only enforced on a per
		// apiserver process basis. With apiserver replicas in place, concurrent access in this code path can
		// still occur. The change here will still limit the number of db connections from here at a time, but they
		// will not necessarily be serialized.
		canceled := limiters.Single.Do(c, func() {
			cachedUserId, err = o.Api.Redis.Get(c.Request.Context(), prefixId).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					o.Logger.Debugf("user id doesn't exits in the cache:%s", err)
				} else {
					o.Logger.Warnf("failed to find user in the cache:%s", err)
				}
			}

			if cachedUserId == "" {
				userId, err := o.Api.CreateUserIfNotExists(c.Request.Context(), idpUserID, idpUserName)
				if err != nil {
					o.Api.SendInternalServerError(c, err)
					c.Abort()
					return
				}
				cachedUserId = userId.String()
				o.Api.Redis.Set(c.Request.Context(), prefixId, userId.String(), handlers.CacheExp)
			}
		})

		if canceled {
			return
		}

		userID, err := uuid.Parse(cachedUserId)
		if err != nil {
			o.Api.SendInternalServerError(c, fmt.Errorf("invalid redis entry for '%s': %w", prefixId, err))
			c.Abort()
			return
		}
		c.Set(gin.AuthUserKey, userID)
		c.Set(AuthUserName, idpUserName)

		logger.Debugf("user-id is %s", idpUserID)
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
