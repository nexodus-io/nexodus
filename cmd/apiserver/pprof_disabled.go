//go:build !pprof

package main

import (
	"context"
	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
)

func pprof_init(ctx context.Context, _ *cli.Command, _ *zap.Logger) {
	// nothing to do here, pprof disabled
}
