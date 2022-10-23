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
	streamPort = 6379
	acLogEnv   = "APEX_CONTROLLER_LOGLEVEL"
)

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
				Name:     "streamer-address",
				Value:    "",
				Usage:    "address of message bus",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "streamer-password",
				Value:    "",
				Usage:    "password of message bus",
				Required: true,
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
				cCtx.String("streamer-address"),
				streamPort,
				cCtx.String("streamer-password"),
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
