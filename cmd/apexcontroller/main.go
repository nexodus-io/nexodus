package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/redhat-et/apex/internal/database"
	"github.com/redhat-et/apex/internal/handlers"
	"github.com/redhat-et/apex/internal/ipam"
	"github.com/redhat-et/apex/internal/routers"
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
			db, err := database.NewDatabase(
				cCtx.String("db-address"),
				"controller",
				cCtx.String("db-password"),
				"controller",
				5432,
				"disable",
			)
			if err != nil {
				log.Fatal(err)
			}

			ipam := ipam.NewIPAM(cCtx.String("ipam-address"))

			api, err := handlers.NewAPI(cCtx.Context, db, ipam)
			if err != nil {
				log.Fatal(err)
			}

			router, err := routers.NewRouter(api, cCtx.String("keycloak-address"))
			if err != nil {
				log.Fatal(err)
			}

			server := &http.Server{
				Addr:    "0.0.0.0:8080",
				Handler: router,
			}

			go func() {
				_ = server.ListenAndServe()
			}()

			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
			<-ch

			return server.Close()
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
