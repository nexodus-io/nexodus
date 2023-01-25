package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/redhat-et/apex/internal/apex"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const (
	apexLogEnv = "APEX_LOGLEVEL"
)

func main() {
	// set the log level
	debug := os.Getenv(apexLogEnv)
	var logger *zap.Logger
	var err error
	if debug != "" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal(err)
	}

	// flags are stored in the global flags variable
	app := &cli.App{
		Name:  "apex",
		Usage: "Node agent to configure encrypted mesh networking.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "public-key",
				Value:    "",
				Usage:    "public key for the local host - agent generates keys by default",
				EnvVars:  []string{"APEX_PUB_KEY"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "private-key",
				Value:    "",
				Usage:    "private key for the local host (dev purposes only - soon to be removed)",
				EnvVars:  []string{"APEX_PRIVATE_KEY"},
				Required: false,
			},
			&cli.IntFlag{
				Name:     "listen-port",
				Value:    0,
				Usage:    "port wireguard is to listen for incoming peers on",
				EnvVars:  []string{"APEX_LISTEN_PORT"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "request-ip",
				Value:    "",
				Usage:    "request a specific IP address from Ipam if available (optional)",
				EnvVars:  []string{"APEX_REQUESTED_IP"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "local-endpoint-ip",
				Value:    "",
				Usage:    "specify the endpoint address of this node instead of being discovered (optional)",
				EnvVars:  []string{"APEX_LOCAL_ENDPOINT_IP"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "child-prefix",
				Value:    "",
				Usage:    "request a CIDR range of addresses that will be advertised from this node (optional)",
				EnvVars:  []string{"APEX_REQUESTED_CHILD_PREFIX"},
				Required: false,
			},
			&cli.BoolFlag{Name: "stun",
				Usage:    "discover the public address for this host using STUN",
				Value:    false,
				EnvVars:  []string{"APEX_STUN"},
				Required: false,
			},
			&cli.BoolFlag{Name: "hub-router",
				Usage:    "set if this node is to be the hub in a hub and spoke deployment",
				Value:    false,
				EnvVars:  []string{"APEX_HUB_ROUTER"},
				Required: false,
			},
			&cli.BoolFlag{Name: "relay-only",
				Usage:    "set if this node is unable to NAT hole punch in a hub zone (Apex will set this automatically if symmetric NAT is detected)",
				Value:    false,
				EnvVars:  []string{"APEX_RELAY_ONLY"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "with-token",
				Value:    "",
				Usage:    "access token for apex controller (optional)",
				EnvVars:  []string{"APEX_ACCESS_TOKEN"},
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
				log.Fatal("<controller-url> required")
			}

			apex, err := apex.NewApex(
				context.Background(), logger.Sugar(),
				controller,
				cCtx.String("with-token"),
				cCtx.Int("listen-port"),
				cCtx.String("public-key"),
				cCtx.String("private-key"),
				cCtx.String("request-ip"),
				cCtx.String("local-endpoint-ip"),
				cCtx.String("child-prefix"),
				cCtx.Bool("stun"),
				cCtx.Bool("hub-router"),
				cCtx.Bool("relay-only"),
			)
			if err != nil {
				log.Fatal(err)
			}

			if err := apex.Run(); err != nil {
				log.Fatal(err)
			}

			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
			<-ch

			if err := apex.Shutdown(context.Background()); err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
