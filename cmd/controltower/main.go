package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/redhat-et/jaywalking/internal/controltower"
	log "github.com/sirupsen/logrus"
)

var (
	streamService *string
	streamPasswd  *string
)

const (
	DefaultIpamSaveFile = "default-ipam.json"
	streamPort          = 6379
	ctLogEnv            = "CONTROLTOWER_LOG_LEVEL"
)

func init() {
	streamService = flag.String("streamer-address", "", "streamer address")
	streamPasswd = flag.String("streamer-passwd", "", "streamer password")
	flag.Parse()
	// set the log level
	env := os.Getenv(ctLogEnv)
	if env == "debug" {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	ct, err := controltower.NewControlTower(
		context.Background(),
		*streamService,
		streamPort,
		*streamPasswd,
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
}
