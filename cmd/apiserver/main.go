package main

import (
	"context"
	"crypto/tls"
	"fmt"
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	redisStore "github.com/go-session/redis/v3"
	"github.com/go-session/session/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/nexodus-io/nexodus/internal/email"
	"github.com/nexodus-io/nexodus/internal/ipam/cmd"
	"github.com/nexodus-io/nexodus/internal/signalbus"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

	"github.com/urfave/cli/v3"
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("apiserver")
}

// @title               Nexodus API
// @description         This is the Nexodus API Server.
// @version             1.0
// @contact.name        The Nexodus Authors
// @contact.url         https://github.com/nexodus-io/nexodus/issues
// @license.name        Apache 2.0
// @license.url         http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath            /

// @tag.name            CA
// @tag.description     X509 Certificate related APIs, these APIs are experimental and disabled by default.  Use the feature flag apis to check if they are enabled on the server.
// @tag.name            Sites
// @tag.description     Skupper Site related APIs, these APIs are experimental and disabled by default.  Use the feature flag apis to check if they are enabled on the server.

// @securitydefinitions.oauth2.implicit OAuth2Implicit
// @authorizationurl     https://auth.try.nexodus.127.0.0.1.nip.io/
// @scope.admin Grants read and write access to administrative information
// @scope.user Grants read and write access to resources owned by this user
func main() {
	// Override to capitalize "Show"
	cli.HelpFlag.(*cli.BoolFlag).Usage = "Show help"
	app := &cli.Command{
		Name: "apiserver",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Value:   false,
				Usage:   "Enable debug logging",
				Sources: cli.EnvVars("NEXAPI_DEBUG"),
			},
			&cli.StringFlag{
				Name:    "listen",
				Value:   "0.0.0.0:8080",
				Usage:   "The address and port to listen for HTTP requests on",
				Sources: cli.EnvVars("NEXAPI_LISTEN"),
			},

			&cli.StringFlag{
				Name:    "listen-grpc",
				Value:   "0.0.0.0:5080",
				Usage:   "The address and port to listen for GRPC requests on",
				Sources: cli.EnvVars("NEXAPI_LISTEN_GRPC"),
			},

			&cli.StringFlag{
				Name:    "oidc-url",
				Value:   "https://auth.try.nexodus.127.0.0.1.nip.io",
				Usage:   "Address of oidc provider",
				Sources: cli.EnvVars("NEXAPI_OIDC_URL"),
			},
			&cli.StringFlag{
				Name:    "oidc-backchannel-url",
				Value:   "",
				Usage:   "Backend address of oidc provider",
				Sources: cli.EnvVars("NEXAPI_OIDC_BACKCHANNEL"),
			},
			&cli.BoolFlag{
				Name:    "insecure-tls",
				Value:   false,
				Usage:   "Trust any TLS certificate",
				Sources: cli.EnvVars("NEXAPI_INSECURE_TLS"),
			},
			&cli.StringFlag{
				Name:    "oidc-client-id-web",
				Value:   "nexodus-web",
				Usage:   "OIDC client id for web",
				Sources: cli.EnvVars("NEXAPI_OIDC_CLIENT_ID_WEB"),
			},
			&cli.StringFlag{
				Name:    "oidc-client-secret-web",
				Value:   "",
				Usage:   "OIDC client secret for web",
				Sources: cli.EnvVars("NEXAPI_OIDC_CLIENT_SECRET_WEB"),
			},
			&cli.StringFlag{
				Name:    "oidc-client-id-cli",
				Value:   "nexodus-cli",
				Usage:   "OIDC client id for cli",
				Sources: cli.EnvVars("NEXAPI_OIDC_CLIENT_ID_CLI"),
			},
			&cli.StringFlag{
				Name:    "db-host",
				Value:   "apiserver-db",
				Usage:   "Database host name",
				Sources: cli.EnvVars("NEXAPI_DB_HOST"),
			},
			&cli.StringFlag{
				Name:    "db-port",
				Value:   "5432",
				Usage:   "Database port",
				Sources: cli.EnvVars("NEXAPI_DB_PORT"),
			},
			&cli.StringFlag{
				Name:    "db-user",
				Value:   "apiserver",
				Usage:   "Database user",
				Sources: cli.EnvVars("NEXAPI_DB_USER"),
			},
			&cli.StringFlag{
				Name:    "db-password",
				Value:   "secret",
				Usage:   "Database password",
				Sources: cli.EnvVars("NEXAPI_DB_PASSWORD"),
			},
			&cli.StringFlag{
				Name:    "db-name",
				Value:   "apiserver",
				Usage:   "Database name",
				Sources: cli.EnvVars("NEXAPI_DB_NAME"),
			},
			&cli.StringFlag{
				Name:    "db-sslmode",
				Value:   "disable",
				Usage:   "Database ssl mode",
				Sources: cli.EnvVars("NEXAPI_DB_SSLMODE"),
			},
			&cli.StringFlag{
				Name:    "ipam-address",
				Value:   "ipam:9090",
				Usage:   "Address of ipam grpc service",
				Sources: cli.EnvVars("NEXAPI_IPAM_URL"),
			},
			&cli.BoolFlag{
				Name:    "trace-insecure",
				Value:   false,
				Usage:   "Set OTLP endpoint to insecure mode",
				Sources: cli.EnvVars("NEXAPI_TRACE_INSECURE"),
			},
			&cli.StringFlag{
				Name:    "trace-endpoint",
				Value:   "",
				Usage:   "OTLP endpoint for trace data",
				Sources: cli.EnvVars("NEXAPI_TRACE_ENDPOINT_OTLP"),
			},
			&cli.StringSliceFlag{
				Name:    "scopes",
				Usage:   "Additional OAUTH2 scopes",
				Sources: cli.EnvVars("NEXAPI_SCOPES"),
			},
			&cli.StringSliceFlag{
				Name:    "origins",
				Usage:   "Trusted Origins. At least 1 MUST be provided",
				Sources: cli.EnvVars("NEXAPI_ORIGINS"),
			},
			&cli.StringFlag{
				Name:    "domain",
				Usage:   "Domain that the agent is running on.",
				Value:   "api.example.com",
				Sources: cli.EnvVars("NEXAPI_DOMAIN"),
			},
			&cli.StringFlag{
				Name:    "cookie-key",
				Usage:   "Key to the cookie jar.",
				Value:   "p2s5v8y/B?E(G+KbPeShVmYq3t6w9z$C",
				Sources: cli.EnvVars("NEXAPI_COOKIE_KEY"),
			},
			&cli.StringFlag{
				Name:     "redis-server",
				Usage:    "Redis host:port address",
				Value:    "redis:6379",
				Sources:  cli.EnvVars("NEXAPI_REDIS_SERVER"),
				Required: true,
			},
			&cli.IntFlag{
				Name:    "redis-db",
				Usage:   "Redis database to be selected after connecting to the server.",
				Value:   1,
				Sources: cli.EnvVars("NEXAPI_REDIS_DB"),
			},
			&cli.StringFlag{
				Name:     "tls-key",
				Usage:    "The server jwks private key",
				Required: true,
				Sources:  cli.EnvVars("NEXAPI_TLS_KEY"),
			},
			&cli.StringFlag{
				Name:     "tls-cert",
				Usage:    "The server jwks cert key",
				Required: true,
				Sources:  cli.EnvVars("NEXAPI_TLS_KEY"),
			},
			&cli.StringFlag{
				Name:     "url",
				Usage:    "The server url",
				Required: true,
				Sources:  cli.EnvVars("NEXAPI_URL"),
			},

			&cli.StringFlag{
				Name:     "smtp-host-port",
				Usage:    "SMTP server host:port address",
				Required: false,
				Sources:  cli.EnvVars("NEXAPI_SMTP_HOST_PORT"),
			},
			&cli.StringFlag{
				Name:     "smtp-user",
				Usage:    "SMTP server user name",
				Required: false,
				Sources:  cli.EnvVars("NEXAPI_SMTP_USER"),
			},
			&cli.StringFlag{
				Name:     "smtp-password",
				Usage:    "SMTP server password",
				Required: false,
				Sources:  cli.EnvVars("NEXAPI_SMTP_PASSWORD"),
			},
			&cli.BoolFlag{
				Name:     "smtp-tls",
				Usage:    "Use TLS to connect to the SMTP server",
				Required: false,
				Sources:  cli.EnvVars("NEXAPI_SMTP_TLS"),
			},
			&cli.StringFlag{
				Name:     "smtp-from",
				Usage:    "The from address to use for emails",
				Required: false,
				Sources:  cli.EnvVars("NEXAPI_SMTP_FROM"),
			},
			&cli.StringFlag{
				Name:     "ca-cert",
				Usage:    "Certificate authority cert",
				Required: false,
				Sources:  cli.EnvVars("NEXAPI_CA_CERT"),
			},
			&cli.StringFlag{
				Name:     "ca-key",
				Usage:    "Certificate authority key",
				Required: false,
				Sources:  cli.EnvVars("NEXAPI_CA_KEY"),
			},
		},

		Action: func(ctx context.Context, command *cli.Command) error {
			ctx, _ = signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
			ctx, span := tracer.Start(ctx, "Run")
			defer span.End()
			withLoggerAndDB(ctx, command, func(logger *zap.Logger, db *gorm.DB, dsn string) {
				pprof_init(ctx, command, logger)

				if err := database.Migrations().Migrate(ctx, db); err != nil {
					log.Fatal(err)
				}

				signalBus := signalbus.NewPgSignalBus(signalbus.NewSignalBus(), db, dsn, logger.Sugar())
				wg := &sync.WaitGroup{}
				signalBus.Start(ctx, wg)

				ipam := ipam.NewIPAM(logger.Sugar(), command.String("ipam-address"))

				fflags := fflags.NewFFlags(logger.Sugar())

				store := inmem.New()

				redisClient := redis.NewClient(&redis.Options{
					Addr: command.String("redis-server"),
					DB:   int(command.Int("redis-db")),
				})

				sessionStore := redisStore.NewRedisStore(&redisStore.Options{
					Addr: command.String("redis-server"),
					DB:   int(command.Int("redis-db")),
				})

				sessionManager := session.NewManager(
					session.SetCookieName(handlers.SESSION_ID_COOKIE_NAME),
					session.SetStore(sessionStore),
				)

				caKeyPair := handlers.CertificateKeyPair{}
				if command.String("ca-cert") != "" && command.String("ca-key") != "" {
					var err error
					caKeyPair, err = handlers.ParseCertificateKeyPair([]byte(command.String("ca-cert")), []byte(command.String("ca-key")))
					if err != nil {
						log.Fatal("invalid --ca-cert or --ca-key values:", err)
					}
				}

				api, err := handlers.NewAPI(ctx, logger.Sugar(), db, ipam, fflags, store, signalBus, redisClient, sessionManager, caKeyPair)
				if err != nil {
					log.Fatal(err)
				}

				api.URL = command.String("url")
				api.URLParsed, err = url.Parse(api.URL)
				if err != nil {
					log.Fatal(fmt.Errorf("invalid url: %w", err))
				}

				smtpServer := email.SmtpServer{
					HostPort: command.String("smtp-host-port"),
					User:     command.String("smtp-user"),
					Password: command.String("smtp-password"),
				}
				if command.Bool("smtp-tls") { // #nosec G402
					smtpServer.Tls = &tls.Config{
						InsecureSkipVerify: command.Bool("insecure-tls"),
					}
				}
				api.SmtpServer = smtpServer
				api.SmtpFrom = command.String("smtp-from")

				scopes := []string{"openid", "profile", "email"}
				scopes = append(scopes, command.StringSlice("scopes")...)

				webAuth, err := agent.NewOidcAgent(
					ctx,
					logger,
					command.String("oidc-url"),
					command.String("oidc-backchannel-url"),
					command.Bool("insecure-tls"),
					command.String("oidc-client-id-web"),
					command.String("oidc-client-secret-web"),
					fmt.Sprintf("%s/web/login/end", api.URL),
					scopes,
					command.String("domain"),
					command.StringSlice("origins"),
					"", // backend
					command.String("cookie-key"),
				)
				if err != nil {
					log.Fatal(err)
				}

				cliAuth, err := agent.NewOidcAgent(
					ctx,
					logger,
					command.String("oidc-url"),
					command.String("oidc-backchannel-url"),
					command.Bool("insecure-tls"),
					command.String("oidc-client-id-cli"),
					"", // clientSecret
					"", // redirectURL
					scopes,
					command.String("domain"),
					[]string{}, // origins
					"",         // backend
					"",         // cookieKey
				)
				if err != nil {
					log.Fatal(err)
				}

				tlsKey := command.String("tls-key")
				api.PrivateKey, err = jwt.ParseRSAPrivateKeyFromPEM([]byte(tlsKey))
				if err != nil {
					log.Fatal(fmt.Errorf("invalid tls-key: %w", err))
				}

				router, err := routers.NewAPIRouter(ctx, routers.APIRouterOptions{
					Logger:          logger.Sugar(),
					Api:             api,
					ClientIdWeb:     command.String("oidc-client-id-web"),
					ClientIdCli:     command.String("oidc-client-id-cli"),
					OidcURL:         command.String("oidc-url"),
					OidcBackchannel: command.String("oidc-backchannel-url"),
					InsecureTLS:     command.Bool("insecure-tls"),
					BrowserFlow:     webAuth,
					DeviceFlow:      cliAuth,
					Store:           store,
					SessionStore:    sessionStore,
				})
				if err != nil {
					log.Fatal(err)
				}

				httpServer := &http.Server{
					Addr:              command.String("listen"),
					Handler:           router,
					ReadTimeout:       5 * time.Second,
					ReadHeaderTimeout: 5 * time.Second,
					WriteTimeout:      10 * time.Second,
				}
				defer util.IgnoreError(httpServer.Close)

				serveErrors := make(chan error, 2)
				util.GoWithWaitGroup(wg, func() {
					if err = httpServer.ListenAndServe(); err != nil {
						serveErrors <- err
					}
				})

				grpcListener, err := net.Listen("tcp", command.String("listen-grpc"))
				if err != nil {
					log.Fatal(err)
				}
				defer util.IgnoreError(grpcListener.Close)

				grpcServer := grpc.NewServer()
				defer grpcServer.Stop()
				auth.RegisterAuthorizationServer(grpcServer, api)
				util.GoWithWaitGroup(wg, func() {
					if err = grpcServer.Serve(grpcListener); err != nil {
						serveErrors <- err
					}
				})

				// Wait for a shutdown signal or a server has an error
				beginShutdown := &sync.WaitGroup{}
				util.GoWithWaitGroup(beginShutdown, func() {
					select {
					case err := <-serveErrors:
						serveErrors <- err // put it back
					case <-ctx.Done():
					}
				})
				beginShutdown.Wait()

				// Try to do a graceful shutdown of the servers for 5 seconds...
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				go func() {
					grpcServer.GracefulStop()
				}()
				go func() {
					_ = httpServer.Shutdown(shutdownCtx)
				}()

				serversDone := make(chan struct{})
				go func() {
					wg.Wait()
					close(serversDone)
				}()

				// Wait for both servers to gracefully shutdown or timeout...
				err = nil
			forLoop:
				for {
					select {
					case err = <-serveErrors: // save any errors
					case <-shutdownCtx.Done():
						break forLoop
					case <-serversDone:
						break forLoop
					}
				}

				if err != nil {
					log.Fatal(err)
				}
			})
			return nil
		},
	}
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "rollback",
		Usage: "Rollback the last database migration",
		Action: func(ctx context.Context, command *cli.Command) error {

			withLoggerAndDB(ctx, command, func(logger *zap.Logger, db *gorm.DB, dsn string) {
				if err := database.Migrations().RollbackLast(ctx, db); err != nil {
					log.Fatal(err)
				}
			})
			return nil
		},
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name: "ipam",
		// only show this sub command if your in debug mode.
		Hidden: os.Getenv("NEXAPI_DEBUG") != "true",
		Usage:  "Interact with the ipam service",
		Commands: []*cli.Command{
			{
				Name:  "rebuild",
				Usage: "Rebuild the IPAM service using the allocated ips and cidrs in nexodus database",
				Action: func(ctx context.Context, command *cli.Command) error {

					withLoggerAndDB(ctx, command, func(logger *zap.Logger, db *gorm.DB, dsn string) {
						ipam := ipam.NewIPAM(logger.Sugar(), command.String("ipam-address"))
						if err := cmd.Rebuild(ctx, logger, db, ipam); err != nil {
							log.Fatal(err)
						}
					})
					return nil
				},
			},
			{
				Name:  "clear",
				Usage: "Clear the IPAM db",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "ipam-db-host",
						Value:   "postgres",
						Usage:   "Database host name",
						Sources: cli.EnvVars("NEXAPI_DB_HOST"),
					},
					&cli.StringFlag{
						Name:    "ipam-db-port",
						Value:   "5432",
						Usage:   "Database port",
						Sources: cli.EnvVars("NEXAPI_DB_PORT"),
					},
					&cli.StringFlag{
						Name:    "ipam-db-user",
						Value:   "ipam",
						Usage:   "Database user",
						Sources: cli.EnvVars("IPAM_DB_USER"),
					},
					&cli.StringFlag{
						Name:    "ipam-db-password",
						Value:   "password",
						Usage:   "Database password",
						Sources: cli.EnvVars("IPAM_DB_PASSWORD"),
					},
					&cli.StringFlag{
						Name:    "ipam-db-name",
						Value:   "ipam",
						Usage:   "Database name",
						Sources: cli.EnvVars("IPAM_DB_NAME"),
					},
					&cli.StringFlag{
						Name:    "ipam-db-sslmode",
						Value:   "disable",
						Usage:   "Database ssl mode",
						Sources: cli.EnvVars("IPAM_DB_SSLMODE"),
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {

					log := getLogger(command).Sugar()
					ipamDB, _, err := database.NewDatabase(
						ctx,
						log,
						command.String("ipam-db-host"),
						command.String("ipam-db-user"),
						command.String("ipam-db-password"),
						command.String("ipam-db-name"),
						command.String("ipam-db-port"),
						command.String("ipam-db-sslmode"),
					)
					if err != nil {
						log.Fatal(err)
					}
					if err := cmd.ClearIpamDB(log, ipamDB); err != nil {
						log.Fatal(err)
					}
					return nil
				},
			},
		},
	})

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func getLogger(command *cli.Command) *zap.Logger {
	var logger *zap.Logger
	var err error
	// set the log level
	if command.Bool("debug") {
		logConfig := zap.NewProductionConfig()
		logConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		logger, err = logConfig.Build()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal(err)
	}
	return logger
}
func withLoggerAndDB(ctx context.Context, command *cli.Command, f func(logger *zap.Logger, db *gorm.DB, dsn string)) {
	logger := getLogger(command)
	cleanup := initTracer(logger.Sugar(), command.Bool("trace-insecure"), command.String("trace-endpoint"))
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
		command.String("db-host"),
		command.String("db-user"),
		command.String("db-password"),
		command.String("db-name"),
		command.String("db-port"),
		command.String("db-sslmode"),
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
