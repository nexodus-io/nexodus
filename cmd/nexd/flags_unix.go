//go:build linux || darwin

package main

import (
	"github.com/nexodus-io/nexodus/internal/api"
	"github.com/urfave/cli/v2"
	"os/user"
	"path/filepath"
)

var stateDirDefault = "/var/lib/nexd"
var stateDirDefaultExpression = "/var/lib/nexd"

func init() {

	currentUser, err := user.Current()
	if err == nil && currentUser.Uid != "0" && currentUser.HomeDir != "" {
		stateDirDefault = filepath.Join(currentUser.HomeDir, ".nexodus")
		stateDirDefaultExpression = "$HOME/.nexodus"
		api.UnixSocketPath = filepath.Join(stateDirDefault, "nexd.sock")
		api.UnixSocketPathExpression = "$HOME/.nexodus/nexd.sock"
	}

	additionalPlatformFlags = append(additionalPlatformFlags,
		&cli.StringFlag{
			Name:        "unix-socket",
			Usage:       "Path to the unix socket nexd is listening against",
			Value:       api.UnixSocketPath,
			Destination: &api.UnixSocketPath,
			DefaultText: api.UnixSocketPathExpression,
			Required:    false,
		})
}
