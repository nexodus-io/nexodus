//go:build linux || darwin

package main

import (
	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/urfave/cli/v2"
)

func init() {
	additionalPlatformFlags = append(additionalPlatformFlags,
		&cli.StringFlag{
			Name:        "unix-socket",
			Usage:       "Path to the unix socket nexd is listening against",
			Value:       nexodus.UnixSocketPath,
			Destination: &nexodus.UnixSocketPath,
			Required:    false,
		})
}
