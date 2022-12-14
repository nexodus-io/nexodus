package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/redhat-et/apex/internal/database"
	"github.com/redhat-et/apex/internal/fflags"
	"github.com/redhat-et/apex/internal/handlers"
	"github.com/redhat-et/apex/internal/ipam"
	"github.com/redhat-et/apex/internal/routers"

	//"github.com/thomaspoignant/go-feature-flag/retriever/httpretriever"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

// @title          Apex API
// @version        1.0
// @description	This is the APEX API Server.

// @contact.name   The Apex Authors
// @contact.url    https://github.com/redhat-et/apex/issues

// @license.name  	Apache 2.0
// @license.url   	http://www.apache.org/licenses/LICENSE-2.0.html

// @securitydefinitions.oauth2.implicit OAuth2Implicit
// @scope.admin Grants read and write access to administrative information
// @scope.user Grants read and write access to resources owned by this user

// @BasePath  		/api
func main() {
	app := &cli.App{
		Name: "apex-controller",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Value:   false,
				Usage:   "enable debug logging",
				EnvVars: []string{"APEX_DEBUG"},
			},
			&cli.StringFlag{
				Name:    "oidc-url",
				Value:   "https://auth.apex.local",
				Usage:   "address of oidc provider",
				EnvVars: []string{"APEX_OIDC_URL"},
			},
			&cli.StringFlag{
				Name:    "oidc-client-id-web",
				Value:   "apex-web",
				Usage:   "OIDC client id for web",
				EnvVars: []string{"APEX_OIDC_CLIENT_ID_WEB"},
			},
			&cli.StringFlag{
				Name:    "oidc-client-id-cli",
				Value:   "apex-web",
				Usage:   "OIDC client id for cli",
				EnvVars: []string{"APEX_OIDC_CLIENT_ID_CLI"},
			},
			&cli.StringFlag{
				Name:    "db-host",
				Value:   "apiserver-db",
				Usage:   "db host",
				EnvVars: []string{"APEX_DB_HOST"},
			},
			&cli.StringFlag{
				Name:    "db-port",
				Value:   "5432",
				Usage:   "db port",
				EnvVars: []string{"APEX_DB_PORT"},
			},
			&cli.StringFlag{
				Name:    "db-user",
				Value:   "apiserver",
				Usage:   "db user",
				EnvVars: []string{"APEX_DB_USER"},
			},
			&cli.StringFlag{
				Name:    "db-password",
				Value:   "secret",
				Usage:   "db password",
				EnvVars: []string{"APEX_DB_PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "db-name",
				Value:   "apiserver",
				Usage:   "db name",
				EnvVars: []string{"APEX_DB_NAME"},
			},
			&cli.StringFlag{
				Name:    "ipam-address",
				Value:   "ipam:9090",
				Usage:   "address of ipam grpc service",
				EnvVars: []string{"APEX_IPAM_URL"},
			},
		},
		Action: func(cCtx *cli.Context) error {
			ctx := cCtx.Context
			var logger *zap.Logger
			var err error
			// set the log level
			if cCtx.Bool("debug") {
				logger, err = zap.NewDevelopment()
			} else {
				logger, err = zap.NewProduction()
			}
			if err != nil {
				log.Fatal(err)
			}

			db, err := database.NewDatabase(
				cCtx.String("db-host"),
				cCtx.String("db-user"),
				cCtx.String("db-password"),
				cCtx.String("db-name"),
				cCtx.String("db-port"),
				"disable",
			)
			if err != nil {
				log.Fatal(err)
			}

			ipam := ipam.NewIPAM(logger.Sugar(), cCtx.String("ipam-address"))

			fflags := fflags.NewFFlags(logger.Sugar())

			api, err := handlers.NewAPI(ctx, logger.Sugar(), db, ipam, fflags)
			if err != nil {
				log.Fatal(err)
			}

			router, err := routers.NewAPIRouter(
				ctx,
				logger.Sugar(),
				api,
				cCtx.String("oidc-client-id-web"),
				cCtx.String("oidc-client-id-cli"),
				cCtx.String("oidc-url"),
			)
			if err != nil {
				log.Fatal(err)
			}

			server := &http.Server{
				Addr:    "0.0.0.0:8080",
				Handler: router,
			}

			go func() {
				if err = server.ListenAndServe(); err != nil {
					log.Fatal(err)
				}
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
