package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	controller "github.com/redhat-et/apex/internal/apexcontroller"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	acLogEnv = "APEX_CONTROLLER_LOGLEVEL"
)

// @title          Apex API
// @version        1.0
// @description	This is the APEX API Server.

// @contact.name   The Apex Authors
// @contact.url    https://github.com/redhat-et/apex/issues

// @license.name  	Apache 2.0
// @license.url   	http://www.apache.org/licenses/LICENSE-2.0.html

// @securitydefinitions.oauth2.implicit OAuth2Implicit
// @authorizationurl /auth/realms/controller/protocol/openid-connect/auth
// @scope.admin Grants read and write access to administrative information
// @scope.user Grants read and write access to resources owned by this user

// @BasePath  		/api
func main() {
	// set the log level
	env := os.Getenv(acLogEnv)
	if env == "debug" {
		log.SetLevel(log.DebugLevel)
	}

	app := &cli.App{
		Name: "apex-controller",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "keycloak-address",
				Value:    "keycloak.apex.svc.cluster.local",
				Usage:    "address of keycloak service",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "db-address",
				Value:    "",
				Usage:    "address of db",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "db-password",
				Value:    "",
				Usage:    "password of db",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "ipam-address",
				Value:    "",
				Usage:    "address of ipam grpc service",
				Required: true,
			},
		},
		Action: func(cCtx *cli.Context) error {
			ct, err := controller.NewController(
				context.Background(),
				cCtx.String("keycloak-address"),
				cCtx.String("db-address"),
				cCtx.String("db-password"),
				cCtx.String("ipam-address"),
			)
			if err != nil {
				log.Fatal(err)
			}

			ct.Run()

			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
			<-ch

			if err := ct.Shutdown(context.Background()); err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
