//go:build linux || darwin || windows

package main

import (
	"fmt"
	"net"
	"net/rpc/jsonrpc"

	"github.com/nexodus-io/nexodus/internal/api"
	"github.com/urfave/cli/v2"
)

func init() {
	additionalPlatformCommands = append(additionalPlatformCommands, &cli.Command{
		Name:  "nexd",
		Usage: "Commands for interacting with the local instance of nexd",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "unix-socket",
				Usage:       "Path to the unix socket nexd is listening against",
				Value:       api.UnixSocketPath,
				Destination: &api.UnixSocketPath,
				Required:    false,
			},
		},
		Subcommands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Display the nexd version",
				Action: func(cCtx *cli.Context) error {
					err := cmdLocalVersion(cCtx)
					if err != nil {
						fmt.Printf("%s\n", err)
					}
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "Display the nexd status",
				Action: func(cCtx *cli.Context) error {
					c, err := cmdLocalStatus(cCtx)
					fmt.Printf("%s", c)
					if err != nil {
						fmt.Printf("%s\n", err)
					}
					return nil
				},
			},
			{
				Name:  "get",
				Usage: "Get a value from the local nexd instance",
				Subcommands: []*cli.Command{
					{
						Name:  "tunnelip",
						Usage: "Get the tunnel IP address",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "ipv6",
								Aliases: []string{"6"},
								Usage:   "Get the IPv6 tunnel IP address",
								Value:   false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							var result string
							var err error
							if err := checkVersion(); err != nil {
								return err
							}
							if cCtx.Bool("ipv6") {
								result, err = callNexd("GetTunnelIPv6")
							} else {
								result, err = callNexd("GetTunnelIPv4")
							}
							if err != nil {
								fmt.Printf("%s\n", err)
								return err
							}
							fmt.Printf("%s\n", result)
							return nil
						},
					},
				},
			},
		},
	})
}

func callNexd(method string) (string, error) {
	conn, err := net.Dial("unix", api.UnixSocketPath)
	if err != nil {
		return "", fmt.Errorf("Failed to connect to nexd: %w\n", err)
	}
	defer conn.Close()

	client := jsonrpc.NewClient(conn)

	var result string
	err = client.Call("NexdCtl."+method, nil, &result)
	if err != nil {
		return "", fmt.Errorf("Failed to execute method (%s): %w\n", method, err)
	}

	return result, nil
}

func checkVersion() error {
	result, err := callNexd("Version")
	if err != nil {
		return fmt.Errorf("Failed to get nexd version: %w\n", err)
	}

	if Version != result {
		errMsg := fmt.Sprintf("Version mismatch: nexctl(%s) nexd(%s)\n", Version, result)
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

func cmdLocalVersion(cCtx *cli.Context) error {
	fmt.Printf("nexctl version: %s\n", Version)

	result, err := callNexd("Version")
	if err == nil {
		fmt.Printf("nexd version: %s\n", result)
	}

	return err
}

func cmdLocalStatus(cCtx *cli.Context) (string, error) {
	if err := checkVersion(); err != nil {
		return "", err
	}

	result, err := callNexd("Status")
	if err != nil {
		return "", err
	}

	return result, err
}
