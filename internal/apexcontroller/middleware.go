package apexcontroller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
)

const (
	AuthUserID    = "user-id"
	AuthUserScope = "scope"
)

type KeyCloakAuth struct {
	jwks *keyfunc.JWKS
}

type Claims struct {
	Scope      string `json:"scope"`
	UserID     string `json:"sid"`
	FullName   string `json:"name"`
	UserName   string `json:"preferred_username"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	jwt.RegisteredClaims
}

func NewKeyCloakAuth(url string) (*KeyCloakAuth, error) {
	// Create the keyfunc options. Use an error handler that logs. Refresh the JWKS when a JWT signed by an unknown KID
	// is found or at the specified interval. Rate limit these refreshes. Timeout the initial JWKS refresh request after
	// 10 seconds. This timeout is also used to create the initial context.Context for keyfunc.Get.
	options := keyfunc.Options{
		RefreshErrorHandler: func(err error) {
			log.Printf("There was an error with the jwt.Keyfunc\nError: %s", err.Error())
		},
		RefreshInterval:   time.Hour,
		RefreshRateLimit:  time.Minute * 5,
		RefreshTimeout:    time.Second * 10,
		RefreshUnknownKID: true,
	}

	jwks, err := keyfunc.Get(url, options)
	if err != nil {
		return nil, fmt.Errorf("Failed to create JWKS from resource at the given URL.\nError: %s", err.Error())
	}
	return &KeyCloakAuth{jwks: jwks}, nil
}

func (a *KeyCloakAuth) AuthFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Request.Header.Get("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no Authorization header present"})
			return
		}

		jwtB64, ok := extractTokenFromAuthHeader(header)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unable to get token from header"})
			return
		}

		token, err := jwt.ParseWithClaims(jwtB64, &Claims{}, a.jwks.Keyfunc)
		if err != nil {
			log.Errorf("Failed to parse the JWT. %s", err.Error())
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token is not valid"})
			return
		}

		if claims, ok := token.Claims.(*Claims); ok {
			c.Set(AuthUserID, claims.ID)
			c.Set(AuthUserScope, claims.Scope)
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unable to extract user info from claims"})
			return
		}

		c.Next()
	}
}

func extractTokenFromAuthHeader(val string) (token string, ok bool) {
	authHeaderParts := strings.Split(val, " ")
	if len(authHeaderParts) != 2 || !strings.EqualFold(authHeaderParts[0], "bearer") {
		return "", false
	}
	return authHeaderParts[1], true
}
