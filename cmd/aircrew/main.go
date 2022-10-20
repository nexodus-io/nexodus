package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/redhat-et/apex/internal/aircrew"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	aircrewLogEnv       = "AIRCREW_LOG_LEVEL"
)

func main() {
	// set the log level
	env := os.Getenv(aircrewLogEnv)
	if env == "debug" {
		log.SetLevel(log.DebugLevel)
	}
	// flags are stored in the global flags variable
	app := &cli.App{
		Name: "aircrew",
		Usage: "Node agent to configure encrypted mesh networking.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "public-key",
				Value:       "",
				Usage:       "public key for the local host (required)",
				EnvVars:     []string{"AIRCREW_PUB_KEY"},
				Required: true,
			},
			&cli.StringFlag{
				Name:        "private-key-file",
				Value:       "",
				Usage:       "private key for the local host (recommended - but alternatively pass through --private-key",
				EnvVars:     []string{"AIRCREW_PRIVATE_KEY_FILE"},
				Required: false,
			},
			&cli.StringFlag{
				Name:        "private-key",
				Value:       "",
				Usage:       "private key for the local host (demo purposes only - alternatively pass through --private-key-file",
				EnvVars:     []string{"AIRCREW_PRIVATE_KEY"},
				Required: false,
			},
			&cli.IntFlag{
				Name:        "listen-port",
				Value:       51820,
				Usage:       "port wireguard is to listen for incoming peers on",
				EnvVars:     []string{"AIRCREW_LISTEN_PORT"},
				Required: false,
			},
			&cli.StringFlag{
				Name:        "controller",
				Value:       "",
				Usage:       "address of the controller (required)",
				EnvVars:     []string{"AIRCREW_CONTROLLER"},
				Required: true,
			},
			&cli.StringFlag{
				Name:        "controller-password",
				Value:       "",
				Usage:       "password for the controller (required)",
				EnvVars:     []string{"AIRCREW_CONTROLLER_PASSWD"},
				Required: true,
			},
			&cli.StringFlag{
				Name:        "zone",
				Value:       "controller",
				Usage:       "the tenancy zone the peer is to join - zone needs to be created before joining (optional)",
				EnvVars:     []string{"AIRCREW_ZONE"},
				Required: false,
			},
			&cli.StringFlag{
				Name:        "request-ip",
				Value:       "",
				Usage:       "request a specific IP address from Ipam if available (optional)",
				EnvVars:     []string{"AIRCREW_REQUESTED_IP"},
				Required: false,
			},
			&cli.StringFlag{
				Name:        "local-endpoint-ip",
				Value:       "",
				Usage:       "specify the endpoint address of this node instead of being discovered (optional)",
				EnvVars:     []string{"AIRCREW_LOCAL_ENDPOINT_IP"},
				Required: false,
			},
			&cli.StringFlag{
				Name:        "child-prefix",
				Value:       "",
				Usage:       "request a CIDR range of addresses that will be advertised from this node (optional)",
				EnvVars:     []string{"AIRCREW_REQUESTED_CHILD_PREFIX"},
				Required: false,
			},
			&cli.BoolFlag{Name: "internal-network",
				Usage:       "do not discover the public address for this host, use the local address for peering",
				Value:       false,
				EnvVars:     []string{"AIRCREW_INTERNAL_NETWORK"},
				Required: false,
			},
			&cli.BoolFlag{Name: "hub-router",
				Usage:       "set if this node is to be the hub in a hub and spoke deployment",
				Value:       false,
				EnvVars:     []string{"AIRCREW_HUB_ROUTER"},
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
		Action: func(c *cli.Context) error {
			aircrew, err := aircrew.NewAircrew(
				context.Background(), c)
			if err != nil {
				log.Fatal(err)
			}

			aircrew.Run()

			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
			<-ch

			if err := aircrew.Shutdown(context.Background()); err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
