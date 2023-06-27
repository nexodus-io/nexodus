package main

import (
	"context"
	"github.com/go-session/redis/v3"
	"github.com/go-session/session/v3"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nexodus-io/nexodus/internal/signalbus"
	"github.com/nexodus-io/nexodus/internal/util"

	agent "github.com/nexodus-io/nexodus/pkg/oidcagent"

	"gorm.io/gorm"

	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/fflags"
	"github.com/nexodus-io/nexodus/internal/handlers"
	"github.com/nexodus-io/nexodus/internal/ipam"
	"github.com/nexodus-io/nexodus/internal/routers"
	"github.com/open-policy-agent/opa/storage/inmem"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
// @authorizationurl https://auth.try.nexodus.127.0.0.1.nip.io/
//
// @scope.admin Grants read and write access to administrative information
// @scope.user Grants read and write access to resources owned by this user

// @BasePath  		/
func main() {
	// Override to capitalize "Show"
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
				Name:    "listen",
				Value:   "0.0.0.0:8080",
				Usage:   "The address and port to listen for requests on",
				EnvVars: []string{"NEXAPI_LISTEN"},
			},
			&cli.StringFlag{
				Name:    "oidc-url",
				Value:   "https://auth.try.nexodus.127.0.0.1.nip.io",
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
				Name:    "oidc-client-secret-web",
				Value:   "",
				Usage:   "OIDC client secret for web",
				EnvVars: []string{"NEXAPI_OIDC_CLIENT_SECRET_WEB"},
			},
			&cli.StringFlag{
				Name:    "oidc-client-id-cli",
				Value:   "nexodus-cli",
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

			&cli.StringFlag{
				Name:    "redirect-url",
				Usage:   "Redirect URL. This is the URL of the SPA.",
				Value:   "https://example.com",
				EnvVars: []string{"NEXAPI_REDIRECT_URL"},
			},
			&cli.StringSliceFlag{
				Name:    "scopes",
				Usage:   "Additional OAUTH2 scopes",
				Value:   &cli.StringSlice{},
				EnvVars: []string{"NEXAPI_SCOPES"},
			},
			&cli.StringSliceFlag{
				Name:    "origins",
				Usage:   "Trusted Origins. At least 1 MUST be provided",
				Value:   &cli.StringSlice{},
				EnvVars: []string{"NEXAPI_ORIGINS"},
			},
			&cli.StringFlag{
				Name:    "domain",
				Usage:   "Domain that the agent is running on.",
				Value:   "api.example.com",
				EnvVars: []string{"NEXAPI_DOMAIN"},
			},
			&cli.StringFlag{
				Name:    "cookie-key",
				Usage:   "Key to the cookie jar.",
				Value:   "p2s5v8y/B?E(G+KbPeShVmYq3t6w9z$C",
				EnvVars: []string{"NEXAPI_COOKIE_KEY"},
			},
			&cli.StringFlag{
				Name:     "redis-server",
				Usage:    "Redis host:port address",
				Value:    "redis:6379",
				EnvVars:  []string{"NEXAPI_REDIS_SERVER"},
				Required: true,
			},
			&cli.IntFlag{
				Name:    "redis-db",
				Usage:   "Redis database to be selected after connecting to the server.",
				Value:   1,
				EnvVars: []string{"NEXAPI_REDIS_DB"},
			},
		},

		Action: func(cCtx *cli.Context) error {
			ctx, _ := signal.NotifyContext(cCtx.Context, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
			ctx, span := tracer.Start(ctx, "Run")
			defer span.End()
			withLoggerAndDB(ctx, cCtx, func(logger *zap.Logger, db *gorm.DB, dsn string) {

				if err := database.Migrations().Migrate(ctx, db); err != nil {
					log.Fatal(err)
				}

				signalBus := signalbus.NewPgSignalBus(signalbus.NewSignalBus(), db, dsn, logger.Sugar())
				wg := &sync.WaitGroup{}
				signalBus.Start(ctx, wg)

				ipam := ipam.NewIPAM(logger.Sugar(), cCtx.String("ipam-address"))

				fflags := fflags.NewFFlags(logger.Sugar())

				store := inmem.New()

				sessionStore := redis.NewRedisStore(&redis.Options{
					Addr: cCtx.String("redis-server"),
					DB:   cCtx.Int("redis-db"),
				})

				// In case in the future we want to support cookie based sessions:
				//sessionStore = cookie.NewCookieStore(
				//	cookie.SetHashKey([]byte(cCtx.String("cookie-key"))),
				//)

				sessionManager := session.NewManager(
					session.SetCookieName(handlers.SESSION_ID_COOKIE_NAME),
					session.SetStore(sessionStore),
				)

				api, err := handlers.NewAPI(ctx, logger.Sugar(), db, ipam, fflags, store, signalBus, sessionManager)
				if err != nil {
					log.Fatal(err)
				}
				scopes := []string{"openid", "profile", "email"}
				scopes = append(scopes, cCtx.StringSlice("scopes")...)

				webAuth, err := agent.NewOidcAgent(
					ctx,
					logger,
					cCtx.String("oidc-url"),
					cCtx.String("oidc-backchannel-url"),
					cCtx.Bool("insecure-tls"),
					cCtx.String("oidc-client-id-web"),
					cCtx.String("oidc-client-secret-web"),
					cCtx.String("redirect-url"),
					scopes,
					cCtx.String("domain"),
					cCtx.StringSlice("origins"),
					"", // backend
					cCtx.String("cookie-key"),
				)
				if err != nil {
					log.Fatal(err)
				}

				cliAuth, err := agent.NewOidcAgent(
					ctx,
					logger,
					cCtx.String("oidc-url"),
					cCtx.String("oidc-backchannel-url"),
					cCtx.Bool("insecure-tls"),
					cCtx.String("oidc-client-id-cli"),
					"", // clientSecret
					"", // redirectURL
					scopes,
					cCtx.String("domain"),
					[]string{}, // origins
					"",         // backend
					"",         // cookieKey
				)
				if err != nil {
					log.Fatal(err)
				}

				router, err := routers.NewAPIRouter(ctx, routers.APIRouterOptions{
					Logger:          logger.Sugar(),
					Api:             api,
					ClientIdWeb:     cCtx.String("oidc-client-id-web"),
					ClientIdCli:     cCtx.String("oidc-client-id-cli"),
					OidcURL:         cCtx.String("oidc-url"),
					OidcBackchannel: cCtx.String("oidc-backchannel-url"),
					InsecureTLS:     cCtx.Bool("insecure-tls"),
					BrowserFlow:     webAuth,
					DeviceFlow:      cliAuth,
					Store:           store,
					RedisServer:     cCtx.String("redis-server"),
					RedisDB:         cCtx.Int("redis-db"),
				})
				if err != nil {
					log.Fatal(err)
				}

				server := &http.Server{
					Addr:              cCtx.String("listen"),
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

				util.GoWithWaitGroup(wg, func() {
					<-ctx.Done()
				})
				wg.Wait() // context is canceled via signal.
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
			withLoggerAndDB(ctx, cCtx, func(logger *zap.Logger, db *gorm.DB, dsn string) {
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

func withLoggerAndDB(ctx context.Context, cCtx *cli.Context, f func(logger *zap.Logger, db *gorm.DB, dsn string)) {

	var logger *zap.Logger
	var err error
	// set the log level
	if cCtx.Bool("debug") {
		logConfig := zap.NewProductionConfig()
		logConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		logger, err = logConfig.Build()
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

	db, dsn, err := database.NewDatabase(
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

	f(logger, db, dsn)
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

	deployEnvironment := os.Getenv("NEXAPI_ENVIRONMENT")
	if deployEnvironment == "" {
		deployEnvironment = "development"
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceName("apiserver"),
				semconv.DeploymentEnvironment(deployEnvironment),
			)),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(resources),
		),
	)
	return exporter.Shutdown
}
