package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "apiserver",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "provider",
				Usage: "OIDC Provider URL",
				Value: "http://auth.widgetcorp.local:8080",
			},
			&cli.StringFlag{
				Name:  "client-id",
				Usage: "OIDC Client ID",
				Value: "widgets-app",
			},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(cCtx *cli.Context) error {
	providerArg := cCtx.String("provider")
	clientIDArg := cCtx.String("client-id")

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, providerArg)
	if err != nil {
		return err
	}
	config := &oidc.Config{
		ClientID: clientIDArg,
	}
	verifier := provider.Verifier(config)

	r := gin.Default()

	// Naive JWS Key validation
	r.Use(func(c *gin.Context) {
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

		_, err := verifier.Verify(c.Request.Context(), parts[1])
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// token.Claims() could add the claims to the request

		c.Next()
	})

	// Widgets API
	r.GET("/widgets", func(c *gin.Context) {
		widgetList := []map[string]interface{}{
			{
				"id":   1,
				"name": "foo",
				"type": "bar",
				"baz":  42,
			},
		}
		c.Header("X-Total-Count", "1")
		c.JSON(http.StatusOK, widgetList)
	})

	return http.ListenAndServe("0.0.0.0:8080", r)
}
