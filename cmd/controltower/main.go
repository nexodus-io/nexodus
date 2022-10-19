package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/redhat-et/apex/internal/controltower"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	DefaultIpamSaveFile = "default-ipam.json"
	streamPort          = 6379
	ctLogEnv            = "CONTROLTOWER_LOG_LEVEL"
)

func main() {
	// set the log level
	env := os.Getenv(ctLogEnv)
	if env == "debug" {
		log.SetLevel(log.DebugLevel)
	}

	app := &cli.App{
		Name: "controltower",
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
		},
		Action: func(cCtx *cli.Context) error {
			ct, err := controltower.NewControlTower(
				context.Background(),
				cCtx.String("streamer-address"),
				streamPort,
				cCtx.String("streamer-password"),
				cCtx.String("db-address"),
				cCtx.String("db-password"),
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
