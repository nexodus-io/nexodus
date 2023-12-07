//go:build pprof

package main

import (
	"context"
	"fmt"
	"net/http"
	// #nosec
	_ "net/http/pprof"
	"os"
	"strconv"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func pprof_init(ctx context.Context, command *cli.Context, logger *zap.Logger) {
	port := "8088"
	if envVar := os.Getenv("NEXAPI_PPROF_PORT"); envVar != "" {
		_, err := strconv.Atoi(port)
		if err != nil {
			logger.Sugar().Errorf("NEXAPI_PPROF_PORT environment variable is invalid: %v", err.Error())
		} else {
			port = envVar
		}
	}

	go func() {
		// #nosec
		err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
		if err != nil {
			logger.Sugar().Errorf("http.ListenAndServe error: %v", err.Error())
		}
	}()
}
