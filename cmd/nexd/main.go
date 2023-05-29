package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"sync"
	"syscall"

	"github.com/nexodus-io/nexodus/internal/stun"

	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const (
	nexodusLogEnv     = "NEXD_LOGLEVEL"
	wireguardOptions  = "Wireguard Options"
	agentOptions      = "Agent Options"
	nexServiceOptions = "Nexodus Service Options"
)

type nexdMode int

const (
	nexdModeAgent nexdMode = iota
	nexdModeProxy
	nexdModeRouter
	nexdModeRelay
)

// This variable is set using ldflags at build time. See Makefile for details.
var Version = "dev"

// Optionally set at build time using ldflags
var DefaultServiceURL = "https://try.nexodus.io"

func nexdRun(cCtx *cli.Context, logger *zap.Logger, mode nexdMode) error {
	serviceURL := cCtx.Args().First()
	if serviceURL == "" && DefaultServiceURL != "" {
		logger.Info("No service URL provided, using default service URL", zap.String("url", DefaultServiceURL))
		serviceURL = DefaultServiceURL
	}

	if cCtx.Args().Len() > 1 {
		return fmt.Errorf("nexd only takes one positional argument, the service URL. Additional arguments ignored: %s", cCtx.Args().Tail())
	}

	_, err := nexodus.CtlStatus(cCtx)
	if err == nil {
		return fmt.Errorf("existing nexd service already running")
	}

	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	userspaceMode := false
	relayNode := false
	var childPrefix []string
	switch mode {
	case nexdModeAgent:
		logger.Info("Starting node agent with wireguard driver")
	case nexdModeRouter:
		childPrefix = cCtx.StringSlice("child-prefix")
		logger.Info("Starting node agent with wireguard driver and router function")
	case nexdModeRelay:
		relayNode = true
		logger.Info("Starting relay agent with wireguard driver")
	case nexdModeProxy:
		userspaceMode = true
		logger.Info("Starting in L4 proxy mode")
	}

	stunServers := cCtx.StringSlice("stun-server")
	if stunServers != nil {
		if len(stunServers) < 2 {
			return fmt.Errorf("at least two stun servers are required")
		}
		stun.SetServers(stunServers)
	}

	nex, err := nexodus.NewNexodus(
		logger.Sugar(),
		serviceURL,
		cCtx.String("username"),
		cCtx.String("password"),
		cCtx.Int("listen-port"),
		cCtx.String("public-key"),
		cCtx.String("private-key"),
		cCtx.String("request-ip"),
		cCtx.String("local-endpoint-ip"),
		childPrefix,
		cCtx.Bool("stun"),
		relayNode,
		cCtx.Bool("relay-only"),
		cCtx.Bool("insecure-skip-tls-verify"),
		Version,
		userspaceMode,
		cCtx.String("state-dir"),
		ctx,
		cCtx.String("org-id"),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}

	wg := &sync.WaitGroup{}

	for _, egressRule := range cCtx.StringSlice("egress") {
		_, err := nex.UserspaceProxyAdd(ctx, wg, egressRule, nexodus.ProxyTypeEgress, false)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to add egress proxy rule (%s): %v", egressRule, err))
		}
	}
	for _, ingressRule := range cCtx.StringSlice("ingress") {
		_, err := nex.UserspaceProxyAdd(ctx, wg, ingressRule, nexodus.ProxyTypeIngress, false)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to add ingress proxy rule (%s): %v", ingressRule, err))
		}
	}
	err = nex.LoadProxyRules()
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to load the stored proxy rules: %v", err))
	}

	if err := nex.Start(ctx, wg); err != nil {
		logger.Fatal(err.Error())
	}
	<-ctx.Done()
	nex.Stop()
	wg.Wait()

	return nil
}

var additionalPlatformFlags []cli.Flag = nil

func main() {
	// set the log level
	debug := os.Getenv(nexodusLogEnv)
	var logger *zap.Logger
	var err error
	if debug != "" {
		logger, err = zap.NewDevelopment()
		logger.Info("Debug logging enabled")
	} else {
		logCfg := zap.NewProductionConfig()
		logCfg.DisableStacktrace = true
		logger, err = logCfg.Build()
	}
	if err != nil {
		logger.Fatal(err.Error())
	}

	// Overwrite usage to capitalize "Show"
	cli.HelpFlag.(*cli.BoolFlag).Usage = "Show help"
	// flags are stored in the global flags variable
	app := &cli.App{
		Name:      "nexd",
		Usage:     "Node agent to configure encrypted mesh networking with nexodus.",
		ArgsUsage: "service-url",
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Get the version of nexd",
				Action: func(cCtx *cli.Context) error {
					fmt.Printf("version: %s\n", Version)
					return nil
				},
			},
			{
				Name:  "proxy",
				Usage: "Run nexd as an L4 proxy instead of creating a network interface",
				Action: func(cCtx *cli.Context) error {
					return nexdRun(cCtx, logger, nexdModeProxy)
				},
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
			},
			{
				Name:  "router",
				Usage: "Enable child-prefix function of the node agent to enable prefix forwarding.",
				Action: func(cCtx *cli.Context) error {
					return nexdRun(cCtx, logger, nexdModeRouter)
				},
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "child-prefix",
						Usage:    "Request a `CIDR` range of addresses that will be advertised from this node (optional)",
						EnvVars:  []string{"NEXD_REQUESTED_CHILD_PREFIX"},
						Required: true,
						Action: func(ctx *cli.Context, childPrefixes []string) error {
							for _, prefix := range childPrefixes {
								if err := nexodus.ValidateCIDR(prefix); err != nil {
									return fmt.Errorf("Child prefix CIDRs passed in --child-prefix %s is not valid: %w", prefix, err)
								}
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "relay",
				Usage: "Enable relay and discovery support function for the node agent.",
				Action: func(cCtx *cli.Context) error {
					if runtime.GOOS != nexodus.Linux.String() {
						return fmt.Errorf("Relay node is only supported for Linux Operating System")
					}

					return nexdRun(cCtx, logger, nexdModeRelay)
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "public-key",
				Value:    "",
				Usage:    "Base64 encoded public `key` for the local host - agent generates keys by default",
				EnvVars:  []string{"NEXD_PUB_KEY"},
				Required: false,
				Category: wireguardOptions,
			},
			&cli.StringFlag{
				Name:     "private-key",
				Value:    "",
				Usage:    "Base64 encoded private `key` for the local host (dev purposes only - soon to be removed)",
				EnvVars:  []string{"NEXD_PRIVATE_KEY"},
				Required: false,
				Category: wireguardOptions,
			},
			&cli.IntFlag{
				Name:     "listen-port",
				Value:    0,
				Usage:    "Wireguard `port` to listen on for incoming peers",
				EnvVars:  []string{"NEXD_LISTEN_PORT"},
				Required: false,
				Category: wireguardOptions,
			},
			&cli.StringFlag{
				Name:     "request-ip",
				Value:    "",
				Usage:    "Request a specific `IPv4` address from IPAM if available (optional)",
				EnvVars:  []string{"NEXD_REQUESTED_IP"},
				Required: false,
				Category: wireguardOptions,
				Action: func(ctx *cli.Context, ip string) error {
					if ip != "" {
						if err := nexodus.ValidateIp(ip); err != nil {
							return fmt.Errorf("the IP address passed in --request-ip %s is not valid: %w", ip, err)
						}
						if util.IsIPv6Address(ip) {
							return fmt.Errorf("--request-ip only supports IPv4 addresses")
						}
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:     "local-endpoint-ip",
				Value:    "",
				Usage:    "Specify the endpoint `IP` address of this node instead of being discovered (optional)",
				EnvVars:  []string{"NEXD_LOCAL_ENDPOINT_IP"},
				Required: false,
				Category: wireguardOptions,
				Action: func(ctx *cli.Context, ip string) error {
					if ip != "" {
						if err := nexodus.ValidateIp(ip); err != nil {
							return fmt.Errorf("the IP address passed in --local-endpoint-ip %s is not valid: %w", ip, err)
						}
					}
					return nil
				},
			},
			&cli.BoolFlag{
				Name:     "stun",
				Usage:    "Discover the public address for this host using STUN",
				Value:    false,
				EnvVars:  []string{"NEXD_STUN"},
				Required: false,
				Category: agentOptions,
			},
			&cli.BoolFlag{
				Name:     "relay-only",
				Usage:    "Set if this node is unable to NAT hole punch or you do not want to fully mesh (Nexodus will set this automatically if symmetric NAT is detected)",
				Value:    false,
				EnvVars:  []string{"NEXD_RELAY_ONLY"},
				Required: false,
				Category: agentOptions,
			},
			&cli.StringFlag{
				Name:     "username",
				Value:    "",
				Usage:    "Username `string` for accessing the nexodus service",
				EnvVars:  []string{"NEXD_USERNAME"},
				Required: false,
				Category: nexServiceOptions,
			},
			&cli.StringFlag{
				Name:     "password",
				Value:    "",
				Usage:    "Password `string` for accessing the nexodus service",
				EnvVars:  []string{"NEXD_PASSWORD"},
				Required: false,
				Category: nexServiceOptions,
			},
			&cli.BoolFlag{
				Name:     "insecure-skip-tls-verify",
				Value:    false,
				Usage:    "If true, server certificates will not be checked for validity. This will make your HTTPS connections insecure",
				EnvVars:  []string{"NEXD_INSECURE_SKIP_TLS_VERIFY"},
				Required: false,
				Category: nexServiceOptions,
			},
			&cli.StringFlag{
				Name:     "state-dir",
				Usage:    fmt.Sprintf("Directory to store state in, such as api tokens to reuse after interactive login. Defaults to'%s'", stateDirDefault),
				Value:    stateDirDefault,
				EnvVars:  []string{"NEXD_STATE_DIR"},
				Category: nexServiceOptions,
			},
			&cli.StringSliceFlag{
				Name:     "stun-server",
				Usage:    "stun server to use discover our endpoint address.  At least two are required.",
				EnvVars:  []string{"NEXD_STUN_SERVER"},
				Category: nexServiceOptions,
			},
			&cli.StringFlag{
				Name:     "org-id",
				Usage:    "Organization ID to use when registering with the nexodus service",
				EnvVars:  []string{"NEXD_ORG_ID"},
				Required: false,
				Category: nexServiceOptions,
			},
		},
		Action: func(cCtx *cli.Context) error {
			return nexdRun(cCtx, logger, nexdModeAgent)
		},
	}

	app.Flags = append(app.Flags, additionalPlatformFlags...)
	sort.Slice(app.Flags, func(i, j int) bool {
		return app.Flags[i].Names()[0] < app.Flags[j].Names()[0]
	})

	if err := app.Run(os.Args); err != nil {
		logger.Fatal(err.Error())
	}
}
