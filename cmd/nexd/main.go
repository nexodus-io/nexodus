package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/nexodus-io/nexodus/internal/nexodus"
	"go.uber.org/zap"
)

const (
	nexodusLogEnv = "NEXD_LOGLEVEL"
)

// This variable is set using ldflags at build time. See Makefile for details.
var Version = "dev"

func main() {
	// set the log level
	debug := os.Getenv(nexodusLogEnv)
	var logger *zap.Logger
	var err error
	if debug != "" {
		logger, err = zap.NewDevelopment()
		logger.Info("Debug logging enabled")
	} else {
		logCfg := zap.NewProductionConfig()
		logCfg.DisableStacktrace = true
		logger, err = logCfg.Build()
	}
	if err != nil {
		logger.Fatal(err.Error())
	}

	// flags are stored in the global flags variable
	app := &cli.App{
		Name:  "nexd",
		Usage: "Node agent to configure encrypted mesh networking with nexodus.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "public-key",
				Value:    "",
				Usage:    "public key for the local host - agent generates keys by default",
				EnvVars:  []string{"NEXD_PUB_KEY"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "private-key",
				Value:    "",
				Usage:    "private key for the local host (dev purposes only - soon to be removed)",
				EnvVars:  []string{"NEXD_PRIVATE_KEY"},
				Required: false,
			},
			&cli.IntFlag{
				Name:     "listen-port",
				Value:    0,
				Usage:    "port wireguard is to listen for incoming peers on",
				EnvVars:  []string{"NEXD_LISTEN_PORT"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "request-ip",
				Value:    "",
				Usage:    "request a specific IP address from Ipam if available (optional)",
				EnvVars:  []string{"NEXD_REQUESTED_IP"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "local-endpoint-ip",
				Value:    "",
				Usage:    "specify the endpoint address of this node instead of being discovered (optional)",
				EnvVars:  []string{"NEXD_LOCAL_ENDPOINT_IP"},
				Required: false,
			},
			&cli.StringSliceFlag{
				Name:     "child-prefix",
				Usage:    "request a CIDR range of addresses that will be advertised from this node (optional)",
				EnvVars:  []string{"NEXD_REQUESTED_CHILD_PREFIX"},
				Required: false,
			},
			&cli.BoolFlag{Name: "stun",
				Usage:    "discover the public address for this host using STUN",
				Value:    false,
				EnvVars:  []string{"NEXD_STUN"},
				Required: false,
			},
			&cli.BoolFlag{Name: "hub-router",
				Usage:    "set if this node is to be the hub in a hub and spoke deployment",
				Value:    false,
				EnvVars:  []string{"NEXD_HUB_ROUTER"},
				Required: false,
			},
			&cli.BoolFlag{Name: "relay-only",
				Usage:    "set if this node is unable to NAT hole punch in a hub zone (Nexodus will set this automatically if symmetric NAT is detected)",
				Value:    false,
				EnvVars:  []string{"NEXD_RELAY_ONLY"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "username",
				Value:    "",
				Usage:    "username for accessing the nexodus service",
				EnvVars:  []string{"NEXD_USERNAME"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "password",
				Value:    "",
				Usage:    "password for accessing the nexodus service",
				EnvVars:  []string{"NEXD_PASSWORD"},
				Required: false,
			},
		},
		Before: func(c *cli.Context) error {
			if c.IsSet("clean") {
				log.Print("Cleaning up any existing interfaces")
				// todo: implement a cleanup function
			}
			return nil
		},
		Action: func(cCtx *cli.Context) error {

			controller := cCtx.Args().First()
			if controller == "" {
				logger.Info("<controller-url> required")
				return nil
			}

			ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

			nexodus, err := nexodus.NewNexodus(
				ctx,
				logger.Sugar(),
				controller,
				cCtx.String("username"),
				cCtx.String("password"),
				cCtx.Int("listen-port"),
				cCtx.String("public-key"),
				cCtx.String("private-key"),
				cCtx.String("request-ip"),
				cCtx.String("local-endpoint-ip"),
				cCtx.StringSlice("child-prefix"),
				cCtx.Bool("stun"),
				cCtx.Bool("hub-router"),
				cCtx.Bool("relay-only"),
				Version,
			)
			if err != nil {
				logger.Fatal(err.Error())
			}

			wg := &sync.WaitGroup{}
			if err := nexodus.Start(ctx, wg); err != nil {
				logger.Fatal(err.Error())
			}
			wg.Wait()

			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		logger.Fatal(err.Error())
	}
}
