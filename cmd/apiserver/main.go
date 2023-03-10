package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"

	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/fflags"
	"github.com/nexodus-io/nexodus/internal/handlers"
	"github.com/nexodus-io/nexodus/internal/ipam"
	"github.com/nexodus-io/nexodus/internal/routers"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"

	"github.com/urfave/cli/v2"
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("apiserver")
}

// @title          Nexodus API
// @version        1.0
// @description	This is the Nexodus API Server.

// @contact.name   The Nexodus Authors
// @contact.url    https://github.com/nexodus-io/nexodus/issues

// @license.name  	Apache 2.0
// @license.url   	http://www.apache.org/licenses/LICENSE-2.0.html

// @securitydefinitions.oauth2.implicit OAuth2Implicit
// @authorizationurl https://auth.try.nexodus.local/
//
// @scope.admin Grants read and write access to administrative information
// @scope.user Grants read and write access to resources owned by this user

// @BasePath  		/api
func main() {
	cli.HelpFlag.(*cli.BoolFlag).Usage = "Show help"
	app := &cli.App{
		Name: "nexodus-controller",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Value:   false,
				Usage:   "Enable debug logging",
				EnvVars: []string{"NEXAPI_DEBUG"},
			},
			&cli.StringFlag{
				Name:    "oidc-url",
				Value:   "https://auth.try.nexodus.local",
				Usage:   "Address of oidc provider",
				EnvVars: []string{"NEXAPI_OIDC_URL"},
			},
			&cli.StringFlag{
				Name:    "oidc-backchannel-url",
				Value:   "",
				Usage:   "Backend address of oidc provider",
				EnvVars: []string{"NEXAPI_OIDC_BACKCHANNEL"},
			},
			&cli.BoolFlag{
				Name:    "insecure-tls",
				Value:   false,
				Usage:   "Trust any TLS certificate",
				EnvVars: []string{"NEXAPI_INSECURE_TLS"},
			},
			&cli.StringFlag{
				Name:    "oidc-client-id-web",
				Value:   "nexodus-web",
				Usage:   "OIDC client id for web",
				EnvVars: []string{"NEXAPI_OIDC_CLIENT_ID_WEB"},
			},
			&cli.StringFlag{
				Name:    "oidc-client-id-cli",
				Value:   "nexodus-web",
				Usage:   "OIDC client id for cli",
				EnvVars: []string{"NEXAPI_OIDC_CLIENT_ID_CLI"},
			},
			&cli.StringFlag{
				Name:    "db-host",
				Value:   "apiserver-db",
				Usage:   "Database host name",
				EnvVars: []string{"NEXAPI_DB_HOST"},
			},
			&cli.StringFlag{
				Name:    "db-port",
				Value:   "5432",
				Usage:   "Database port",
				EnvVars: []string{"NEXAPI_DB_PORT"},
			},
			&cli.StringFlag{
				Name:    "db-user",
				Value:   "apiserver",
				Usage:   "Database user",
				EnvVars: []string{"NEXAPI_DB_USER"},
			},
			&cli.StringFlag{
				Name:    "db-password",
				Value:   "secret",
				Usage:   "Database password",
				EnvVars: []string{"NEXAPI_DB_PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "db-name",
				Value:   "apiserver",
				Usage:   "Database name",
				EnvVars: []string{"NEXAPI_DB_NAME"},
			},
			&cli.StringFlag{
				Name:    "db-sslmode",
				Value:   "disable",
				Usage:   "Database ssl mode",
				EnvVars: []string{"NEXAPI_DB_SSLMODE"},
			},
			&cli.StringFlag{
				Name:    "ipam-address",
				Value:   "ipam:9090",
				Usage:   "Address of ipam grpc service",
				EnvVars: []string{"NEXAPI_IPAM_URL"},
			},
			&cli.BoolFlag{
				Name:    "trace-insecure",
				Value:   false,
				Usage:   "Set OTLP endpoint to insecure mode",
				EnvVars: []string{"NEXAPI_TRACE_INSECURE"},
			},
			&cli.StringFlag{
				Name:    "trace-endpoint",
				Value:   "",
				Usage:   "OTLP endpoint for trace data",
				EnvVars: []string{"NEXAPI_TRACE_ENDPOINT_OTLP"},
			},
		},
		Action: func(cCtx *cli.Context) error {
			ctx, span := tracer.Start(cCtx.Context, "Run")
			defer span.End()
			withLoggerAndDB(ctx, cCtx, func(logger *zap.Logger, db *gorm.DB) {

				if err := database.Migrations().Migrate(ctx, db); err != nil {
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
					cCtx.String("oidc-backchannel-url"),
					cCtx.Bool("insecure-tls"),
				)
				if err != nil {
					log.Fatal(err)
				}

				server := &http.Server{
					Addr:              "0.0.0.0:8080",
					Handler:           router,
					ReadTimeout:       5 * time.Second,
					ReadHeaderTimeout: 5 * time.Second,
					WriteTimeout:      10 * time.Second,
				}

				go func() {
					if err = server.ListenAndServe(); err != nil {
						log.Fatal(err)
					}
				}()

				ch := make(chan os.Signal, 1)
				signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
				<-ch

				server.Close()
			})
			return nil
		},
	}
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "rollback",
		Usage: "Rollback the last database migration",
		Action: func(cCtx *cli.Context) error {
			ctx := cCtx.Context
			withLoggerAndDB(ctx, cCtx, func(logger *zap.Logger, db *gorm.DB) {
				if err := database.Migrations().RollbackLast(ctx, db); err != nil {
					log.Fatal(err)
				}
			})
			return nil
		},
	})

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func withLoggerAndDB(ctx context.Context, cCtx *cli.Context, f func(logger *zap.Logger, db *gorm.DB)) {

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

	cleanup := initTracer(logger.Sugar(), cCtx.Bool("trace-insecure"), cCtx.String("trace-endpoint"))
	defer func() {
		if cleanup == nil {
			return
		}
		if err := cleanup(ctx); err != nil {
			logger.Error(err.Error())
		}
	}()

	db, err := database.NewDatabase(
		ctx,
		logger.Sugar(),
		cCtx.String("db-host"),
		cCtx.String("db-user"),
		cCtx.String("db-password"),
		cCtx.String("db-name"),
		cCtx.String("db-port"),
		cCtx.String("db-sslmode"),
	)
	if err != nil {
		log.Fatal(err)
	}

	f(logger, db)
}

func initTracer(logger *zap.SugaredLogger, insecure bool, collector string) func(context.Context) error {
	if collector == "" {
		logger.Info("No collector endpoint configured")
		otel.SetTracerProvider(
			sdktrace.NewTracerProvider(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
			),
		)
		return nil
	}
	secureOption := otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	if insecure {
		secureOption = otlptracegrpc.WithInsecure()
	}
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			secureOption,
			otlptracegrpc.WithEndpoint(collector),
		),
	)
	if err != nil {
		logger.Errorf("Unable to create open telemetry exporter: %s", err.Error())
		return nil
	}
	resources, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", "apiserver"),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		logger.Errorf("Unable to create resources: %s", err.Error())
		return nil
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(resources),
		),
	)
	return exporter.Shutdown
}
