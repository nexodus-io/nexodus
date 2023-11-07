//go:build linux || darwin || windows

package main

import (
	"fmt"
	"net"
	"net/rpc/jsonrpc"
	"path/filepath"

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
				DefaultText: "$HOME/.nexodus/nexd.sock",
				Required:    false,
			},
		},
		Subcommands: []*cli.Command{
			{
				Name:   "version",
				Usage:  "Display the nexd version",
				Action: cmdLocalVersion,
			},
			{
				Name:   "status",
				Usage:  "Display the nexd status",
				Action: cmdLocalStatus,
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
								result, err = callNexd("GetTunnelIPv6", "")
							} else {
								result, err = callNexd("GetTunnelIPv4", "")
							}
							if err != nil {
								fmt.Printf("%s\n", err)
								return err
							}
							fmt.Printf("%s\n", result)
							return nil
						},
					},
					{
						Name:  "debug",
						Usage: "Get the debug logging status",
						Action: func(cCtx *cli.Context) error {
							if err := checkVersion(); err != nil {
								return err
							}
							result, err := callNexd("GetDebug", "")
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
			{
				Name:  "set",
				Usage: "Set a value on the local nexd instance",
				Subcommands: []*cli.Command{
					{
						Name:  "debug",
						Usage: "Set debug logging on or off",
						Subcommands: []*cli.Command{
							{
								Name:  "on",
								Usage: "Turn debug logging on",
								Action: func(cCtx *cli.Context) error {
									if err := checkVersion(); err != nil {
										return err
									}
									result, err := callNexd("SetDebugOn", "")
									if err != nil {
										fmt.Printf("%s\n", err)
										return err
									}
									fmt.Printf("%s\n", result)
									return nil
								},
							},
							{
								Name:  "off",
								Usage: "Turn debug logging off",
								Action: func(cCtx *cli.Context) error {
									if err := checkVersion(); err != nil {
										return err
									}
									result, err := callNexd("SetDebugOff", "")
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
			},
			{
				Name:  "proxy",
				Usage: "Commands for interacting nexd's proxy configuration",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List the nexd proxy rules",
						Action: func(cCtx *cli.Context) error {
							if err := checkVersion(); err != nil {
								return err
							}
							result, err := callNexd("ProxyList", "")
							if err != nil {
								fmt.Printf("%s\n", err)
								return err
							}
							fmt.Printf("%s", result)
							return nil
						},
					},
					{
						Name:  "add",
						Usage: "Add one or more proxy rules to nexd",
						Flags: []cli.Flag{
							&cli.StringSliceFlag{
								Name:     "ingress",
								Usage:    "Forward connections from the Nexodus network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via a locally accessible network using a `value` in the form: protocol:port:destination_ip:destination_port. All fields are required.",
								Required: false,
							},
							&cli.StringSliceFlag{
								Name:     "egress",
								Usage:    "Forward connections from a locally accessible network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via the Nexodus network using a `value` in the form: protocol:port:destination_ip:destination_port. All fields are required.",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							return proxyAddRemove(cCtx, true)
						},
					},
					{
						Name:  "remove",
						Usage: "remove one or more proxy rules to nexd",
						Flags: []cli.Flag{
							&cli.StringSliceFlag{
								Name:     "ingress",
								Usage:    "Forward connections from the Nexodus network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via a locally accessible network using a `value` in the form: protocol:port:destination_ip:destination_port. All fields are required.",
								Required: false,
							},
							&cli.StringSliceFlag{
								Name:     "egress",
								Usage:    "Forward connections from a locally accessible network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via the Nexodus network using a `value` in the form: protocol:port:destination_ip:destination_port. All fields are required.",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							return proxyAddRemove(cCtx, false)
						},
					},
				},
			},
			{
				Name:  "peers",
				Usage: "Commands for interacting with nexd peer connectivity",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list the nexd peers for this device",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "full",
								Aliases: []string{"f"},
								Usage:   "display the full set of peer details",
								Value:   false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							return cmdListPeers(cCtx, encodeOut)
						},
					},
					{
						Name:  "ping",
						Usage: "run a test to check the nexd IPv4 peer connectivity (host firewalls or security groups may block the ICMP probes)",
						Action: func(cCtx *cli.Context) error {
							return cmdConnStatus(cCtx, v4)
						},
					},
					{
						Name:  "ping6",
						Usage: "run a test to check the nexd IPv6 peer connectivity (host firewalls or security groups may block the ICMP probes)",
						Action: func(cCtx *cli.Context) error {
							return cmdConnStatus(cCtx, v6)
						},
					},
				},
			},
			{
				Name:  "exit-node",
				Usage: "Commands for interacting nexd exit node configuration",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list exit nodes",
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							return listExitNodes(cCtx, encodeOut)
						},
					},
					{
						Name:  "enable",
						Usage: "Enable the device to use an exit node in the current organization. Warning: this will funnel all traffic through the exit node if one exists and will likely cause your device to be unreachable outside of the nexodus peer network.",
						Action: func(cCtx *cli.Context) error {
							return enableExitNodeClient(cCtx)
						},
					},
					{
						Name:  "disable",
						Usage: "Disable the device from using an exit node. Traffic will return to using the device's default gateway and direct peers in the nexodus peer network.",
						Flags: []cli.Flag{
							&cli.StringSliceFlag{
								Name:     "client",
								Usage:    "disable the use of an exit node on this device and remove any exit node client configuration if one exists.",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							return disableExitNodeClient(cCtx)
						},
					},
				},
			},
		},
	})
}

func callNexd(method string, arg string) (string, error) {
	conn, err := net.Dial("unix", api.UnixSocketPath)
	if err != nil {
		conn, err = net.Dial("unix", filepath.Base(api.UnixSocketPath))
		if err != nil {
			return "", fmt.Errorf("Failed to connect to nexd: %w\n", err)
		}
	}
	defer conn.Close()

	client := jsonrpc.NewClient(conn)

	var result string
	err = client.Call("NexdCtl."+method, arg, &result)
	if err != nil {
		return "", fmt.Errorf("Failed to execute method (%s): %w\n", method, err)
	}

	return result, nil
}

func checkVersion() error {
	result, err := callNexd("Version", "")
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

	result, err := callNexd("Version", "")
	if err == nil {
		fmt.Printf("nexd version: %s\n", result)
	}

	return err
}

func cmdLocalStatus(cCtx *cli.Context) error {
	if err := checkVersion(); err != nil {
		return err
	}

	result, err := callNexd("Status", "")
	if err != nil {
		return err
	}

	fmt.Printf("%s", result)

	return nil
}

func proxyAddRemove(cCtx *cli.Context, add bool) error {
	if err := checkVersion(); err != nil {
		return err
	}
	ingress := cCtx.StringSlice("ingress")
	egress := cCtx.StringSlice("egress")
	if len(ingress) == 0 && len(egress) == 0 {
		return fmt.Errorf("No rules provided")
	}

	var method string
	addStr := "adding"
	if add {
		method = "ProxyAddIngress"
	} else {
		method = "ProxyRemoveIngress"
		addStr = "removing"
	}
	for _, rule := range ingress {
		result, err := callNexd(method, rule)
		if err != nil {
			fmt.Printf("Error %s ingress rule (%s): %s\n", addStr, rule, err)
			continue
		}
		fmt.Printf("%s", result)
	}
	if add {
		method = "ProxyAddEgress"
	} else {
		method = "ProxyRemoveEgress"
	}
	for _, rule := range egress {
		result, err := callNexd(method, rule)
		if err != nil {
			fmt.Printf("Error %s egress rule (%s): %s\n", addStr, rule, err)
			continue
		}
		fmt.Printf("%s", result)
	}
	return nil
}
