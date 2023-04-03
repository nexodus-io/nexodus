//go:build linux || darwin

package main

import (
	"github.com/nexodus-io/nexodus/internal/api"
	"github.com/urfave/cli/v2"
)

func init() {
	additionalPlatformFlags = append(additionalPlatformFlags,
		&cli.StringFlag{
			Name:        "unix-socket",
			Usage:       "Path to the unix socket nexd is listening against",
			Value:       api.UnixSocketPath,
			Destination: &api.UnixSocketPath,
			Required:    false,
		})
}
