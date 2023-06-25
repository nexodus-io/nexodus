//go:build !pprof

package main

import (
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func pprof_init(_ *cli.Context, _ *zap.Logger) {
	// nothing to do here, pprof disabled
}
