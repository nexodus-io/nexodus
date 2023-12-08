package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/nexodus-io/nexodus/internal/state/fstore"
	"github.com/nexodus-io/nexodus/internal/state/kstore"
	log "github.com/sirupsen/logrus"

	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/nexodus-io/nexodus/internal/stun"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func nexdRun(ctx context.Context, command *cli.Command, logger *zap.Logger, logLevel *zap.AtomicLevel, mode nexdMode) error {

	// Fail if you try to configure the service URL both ways
	if command.IsSet("service-url") && command.Args().Len() > 0 {
		return fmt.Errorf("please remove the service URL positional argument, it was configured via the --service-url flag")
	}
	if command.Args().Len() > 1 {
		return fmt.Errorf("nexd only takes one positional argument, the service URL. Additional arguments ignored: %s", command.Args().Tail())
	}

	serviceURL := ""
	if command.IsSet("service-url") {
		// It was set via a flag
		serviceURL = command.String("service-url")
	} else if command.Args().Len() > 0 {
		// It was set via a positional arg
		serviceURL = command.Args().First()
		logger.Info("DEPRECATION WARNING: configuring the service url via the positional argument will not be supported in a future release.  Please use the --service-url flag instead.")
	}

	regKey := command.String("reg-key")
	if command.IsSet("reg-key") {
		if command.IsSet("security-group-id") {
			return fmt.Errorf("the --reg-key and --security-group-id flags are mutually exclusive")
		}
		if command.IsSet("vpc-id") {
			return fmt.Errorf("the --reg-key and --vpc-id flags are mutually exclusive")
		}

		// TODO: in the future, always assume the service-url is part of the reg-key
		if strings.Contains(regKey, "#") {

			if command.IsSet("service-url") && command.IsSet("reg-key") {
				return fmt.Errorf("the --reg-key and --service-url flags are mutually exclusive")
			}

			u, err := url.Parse(regKey)
			if err != nil {
				return fmt.Errorf("invalid '--reg-key=%s' flag provided. error: %w", regKey, err)
			}
			regKey = u.Fragment
			u.Fragment = ""
			u.RawFragment = ""
			serviceURL = u.String()
		}
	}

	// If it was not set, thctx *cli.Commanden fall back to using the default...
	if serviceURL == "" && DefaultServiceURL != "" {
		logger.Info("No service URL provided, using default service URL", zap.String("url", DefaultServiceURL))
		serviceURL = DefaultServiceURL
	}

	// DefaultServiceURL may not be set... in this case fail since we don't have a service URL
	if serviceURL == "" {
		return fmt.Errorf("no service URL provided: try using the --service-url flag")
	}

	apiURL, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Errorf("invalid '--service-url=%s' flag provided. error: %w", serviceURL, err)
	}

	if apiURL.Scheme != "https" {
		return fmt.Errorf("invalid '--service-url=%s' flag provided. error: 'https://' URL scheme is required", serviceURL)
	}

	// Force controller URL be api.${DOMAIN}
	apiURL.Host = "api." + apiURL.Host
	apiURL.Path = ""

	_, err = nexodus.CtlStatus(command)
	if err == nil {
		return fmt.Errorf("existing nexd service already running")
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	defer cancel()

	pprof_init(ctx, command, logger)

	userspaceMode := false
	relayNode := false
	var advertiseCidr []string
	switch mode {
	case nexdModeAgent:
		logger.Info("Starting node agent with wireguard driver")
	case nexdModeRouter:
		advertiseCidr = command.StringSlice("advertise-cidr")
		// Check if child-prefix is set and log a deprecation warning.
		if command.IsSet("child-prefix") {
			logger.Warn("DEPRECATION WARNING: The 'child-prefix' flag is deprecated. In the future, please use 'advertise-cidr' instead.")
			advertiseCidr = append(advertiseCidr, command.StringSlice("child-prefix")...)
		}
		logger.Info("Starting node agent with wireguard driver and router function")
	case nexdModeRelay:
		relayNode = true
		logger.Info("Starting relay agent with wireguard driver")
	case nexdModeProxy:
		userspaceMode = true
		logger.Info("Starting in L4 proxy mode")
	}

	stunServers := command.StringSlice("stun-server")
	if len(stunServers) > 0 {
		if len(stunServers) < 2 {
			return fmt.Errorf("at least two stun servers are required")
		}
		stun.SetServers(stunServers)
	}

	stateStore, err := kstore.NewIfInCluster()
	if err != nil {
		log.Error(err)
	}

	stateDir := command.String("state-dir")
	if stateStore == nil {
		stateStore = fstore.New(filepath.Join(stateDir, "state.json"))
	}
	defer util.IgnoreError(stateStore.Close)

	options := nexodus.Options{
		Logger:                  logger.Sugar(),
		LogLevel:                logLevel,
		ApiURL:                  apiURL,
		RegKey:                  regKey,
		Username:                command.String("username"),
		Password:                command.String("password"),
		ListenPort:              int(command.Int("listen-port")),
		RequestedIP:             command.String("request-ip"),
		UserProvidedLocalIP:     command.String("local-endpoint-ip"),
		AdvertiseCidrs:          advertiseCidr,
		Relay:                   relayNode,
		RelayOnly:               command.Bool("relay-only"),
		NetworkRouter:           command.Bool("network-router"),
		NetworkRouterDisableNAT: command.Bool("disable-nat"),
		ExitNodeClientEnabled:   command.Bool("exit-node-client"),
		ExitNodeOriginEnabled:   command.Bool("exit-node"),
		InsecureSkipTlsVerify:   command.Bool("insecure-skip-tls-verify"),
		Version:                 Version,
		UserspaceMode:           userspaceMode,
		StateStore:              stateStore,
		StateDir:                stateDir,
		Context:                 ctx,
		VpcId:                   parseUUIDFlag(command, "vpc-id"),
		SecurityGroupId:         parseUUIDFlag(command, "security-group-id"),
	}

	nex, err := nexodus.New(options)
	if err != nil {
		logger.Fatal(err.Error())
	}

	wg := &sync.WaitGroup{}

	for _, egressRule := range command.StringSlice("egress") {
		rule, err := nexodus.ParseProxyRule(egressRule, nexodus.ProxyTypeEgress)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to add egress proxy rule (%s): %v", egressRule, err))
		}
		_, err = nex.UserspaceProxyAdd(rule)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to add egress proxy rule (%s): %v", egressRule, err))
		}
	}
	for _, ingressRule := range command.StringSlice("ingress") {
		rule, err := nexodus.ParseProxyRule(ingressRule, nexodus.ProxyTypeIngress)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to add ingress proxy rule (%s): %v", ingressRule, err))
		}
		_, err = nex.UserspaceProxyAdd(rule)
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

func parseUUIDFlag(command *cli.Command, flagName string) string {
	if !command.IsSet(flagName) {
		return ""
	}
	uuidStr := command.String(flagName)
	uuid, err := uuid.Parse(uuidStr)
	if err != nil {
		log.Fatalf("invalid flag --%s: %s", flagName, err)
	}
	return uuid.String()
}

var additionalPlatformFlags []cli.Flag = nil

func main() {
	// set the log level
	debug := os.Getenv(nexodusLogEnv)
	var logger *zap.Logger
	var logLevel *zap.AtomicLevel
	var err error
	if debug != "" {
		logCfg := zap.NewDevelopmentConfig()
		logLevel = &logCfg.Level
		logger, err = logCfg.Build()
		logger.Info("Debug logging enabled")
	} else {
		logCfg := zap.NewProductionConfig()
		logLevel = &logCfg.Level
		logCfg.DisableStacktrace = true
		logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		logger, err = logCfg.Build()
	}
	if err != nil {
		logger.Fatal(err.Error())
	}

	// Overwrite usage to capitalize "Show"
	cli.HelpFlag.(*cli.BoolFlag).Usage = "Show help"
	// flags are stored in the global flags variable
	app := &cli.Command{
		Name:                  "nexd",
		Usage:                 "Node agent to configure encrypted mesh networking with nexodus.",
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Get the version of nexd",
				Action: func(ctx context.Context, command *cli.Command) error {
					fmt.Printf("version: %s\n", Version)
					return nil
				},
			},
			{
				Name:  "proxy",
				Usage: "Run nexd as an L4 proxy instead of creating a network interface",
				Action: func(ctx context.Context, command *cli.Command) error {
					return nexdRun(ctx, command, logger, logLevel, nexdModeProxy)
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
				Usage: "Enable advertise-cidr function of the node agent to enable prefix forwarding.",
				Action: func(ctx context.Context, command *cli.Command) error {
					if command.Bool("exit-node") {
						if runtime.GOOS != nexodus.Linux.String() {
							return fmt.Errorf("exit-node support is currently only supported for Linux operating systems")
						}
						advertiseCidrs := command.StringSlice("advertise-cidr")
						// Check if "0.0.0.0/0" already exists in advertise-cidr
						found := false
						for _, prefix := range advertiseCidrs {
							if prefix == "0.0.0.0/0" {
								found = true
								break
							}
						}
						// If not found, add it to advertise-cidr
						if !found {
							advertiseCidrs = append(advertiseCidrs, "0.0.0.0/0")
							err := command.Set("advertise-cidr", strings.Join(advertiseCidrs, ","))
							if err != nil {
								return fmt.Errorf("failed to set advertise-cidr: %w", err)
							}
						}
					}
					return nexdRun(ctx, command, logger, logLevel, nexdModeRouter)
				},

				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "advertise-cidr",
						Usage:    "Request a `CIDR` range of addresses that will be advertised from this node (optional)",
						Sources:  cli.EnvVars("NEXD_REQUESTED_ADVERTISE_CIDR"),
						Required: false,
						Action: func(ctx context.Context, command *cli.Command, advertiseCidr []string) error {
							for _, cidr := range advertiseCidr {
								if err := nexodus.ValidateCIDR(cidr); err != nil {
									return fmt.Errorf("advertise prefix CIDR(s) passed in --advertise-cidr %s is not valid: %w", cidr, err)
								}
							}
							return nil
						},
					},
					&cli.StringSliceFlag{
						Name:   "child-prefix",
						Usage:  "(DEPRECATED WARNING) please use --advertise-cidr instead.",
						Hidden: true,
						Action: func(ctx context.Context, command *cli.Command, advertiseCidr []string) error {
							for _, cidr := range advertiseCidr {
								if err := nexodus.ValidateCIDR(cidr); err != nil {
									return fmt.Errorf("advertise prefix CIDR(s) passed in --advertise-cidr %s is not valid: %w", cidr, err)
								}
							}
							return nil
						},
					},
					&cli.BoolFlag{
						Name:     "network-router",
						Usage:    "Make the node a network router node that will forward traffic specified by --advertise-cidr through the physical interface that contains the default gateway",
						Value:    false,
						Sources:  cli.EnvVars("NEXD_NET_ROUTER_NODE"),
						Required: false,
					},
					&cli.BoolFlag{
						Name:     "disable-nat",
						Usage:    "disable NAT for the network router mode. This will require devices on the network to be configured with an ip route",
						Value:    false,
						Sources:  cli.EnvVars("NEXD_DISABLE_NAT"),
						Required: false,
					},
					&cli.BoolFlag{
						Name:     "exit-node",
						Usage:    "Enable this node to be an exit node. This allows other agents to source all traffic leaving the Nexodus mesh from this node",
						Value:    false,
						Sources:  cli.EnvVars("NEXD_EXIT_NODE"),
						Required: false,
					},
				},
			},
			{
				Name:  "relay",
				Usage: "Enable relay and discovery support function for the node agent.",
				Action: func(ctx context.Context, command *cli.Command) error {
					if runtime.GOOS != nexodus.Linux.String() {
						return fmt.Errorf("Relay node is only supported for Linux Operating System")
					}

					return nexdRun(ctx, command, logger, logLevel, nexdModeRelay)
				},
			},
		},
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:       "listen-port",
				Value:      0,
				Usage:      "Wireguard `port` to listen on for incoming peers",
				Sources:    cli.EnvVars("NEXD_LISTEN_PORT"),
				Required:   false,
				Category:   wireguardOptions,
				Persistent: true,
			},
			&cli.StringFlag{
				Name:       "request-ip",
				Value:      "",
				Usage:      "Request a specific `IPv4` address from IPAM if available (optional)",
				Sources:    cli.EnvVars("NEXD_REQUESTED_IP"),
				Required:   false,
				Category:   wireguardOptions,
				Persistent: true,
				Action: func(ctx context.Context, command *cli.Command, ip string) error {
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
				Name:       "local-endpoint-ip",
				Value:      "",
				Usage:      "Specify the endpoint `IP` address of this node instead of being discovered (optional)",
				Sources:    cli.EnvVars("NEXD_LOCAL_ENDPOINT_IP"),
				Required:   false,
				Category:   wireguardOptions,
				Persistent: true,
				Action: func(ctx context.Context, command *cli.Command, ip string) error {
					if ip != "" {
						if err := nexodus.ValidateIp(ip); err != nil {
							return fmt.Errorf("the IP address passed in --local-endpoint-ip %s is not valid: %w", ip, err)
						}
					}
					return nil
				},
			},
			&cli.BoolFlag{
				Name:       "relay-only",
				Usage:      "Set if this node is unable to NAT hole punch or you do not want to fully mesh (Nexodus will set this automatically if symmetric NAT is detected)",
				Value:      false,
				Sources:    cli.EnvVars("NEXD_RELAY_ONLY"),
				Required:   false,
				Category:   agentOptions,
				Persistent: true,
			},
			&cli.StringFlag{
				Name:       "username",
				Value:      "",
				Usage:      "Username `string` for accessing the nexodus service",
				Sources:    cli.EnvVars("NEXD_USERNAME"),
				Required:   false,
				Category:   nexServiceOptions,
				Persistent: true,
			},
			&cli.StringFlag{
				Name:       "password",
				Value:      "",
				Usage:      "Password `string` for accessing the nexodus service",
				Sources:    cli.EnvVars("NEXD_PASSWORD"),
				Required:   false,
				Category:   nexServiceOptions,
				Persistent: true,
			},
			&cli.BoolFlag{
				Name:       "insecure-skip-tls-verify",
				Value:      false,
				Usage:      "If true, server certificates will not be checked for validity. This will make your HTTPS connections insecure",
				Sources:    cli.EnvVars("NEXD_INSECURE_SKIP_TLS_VERIFY"),
				Required:   false,
				Category:   nexServiceOptions,
				Persistent: true,
			},
			&cli.StringFlag{
				Name:        "state-dir",
				Usage:       "Directory to store state in, such as api tokens to reuse after interactive login.",
				Value:       stateDirDefault,
				Sources:     cli.EnvVars("NEXD_STATE_DIR"),
				DefaultText: stateDirDefaultExpression,
				Category:    nexServiceOptions,
				Persistent:  true,
			},
			&cli.StringSliceFlag{
				Name:       "stun-server",
				Usage:      "stun server to use discover our endpoint address.  At least two are required.",
				Sources:    cli.EnvVars("NEXD_STUN_SERVER"),
				Category:   nexServiceOptions,
				Persistent: true,
			},
			&cli.StringFlag{
				Name:       "vpc-id",
				Usage:      "VPC ID to use when registering with the nexodus service",
				Sources:    cli.EnvVars("NEXD_VPC_ID"),
				Required:   false,
				Category:   nexServiceOptions,
				Persistent: true,
			},
			&cli.StringFlag{
				Name:       "service-url",
				Usage:      "URL to the Nexodus service",
				Value:      DefaultServiceURL,
				Sources:    cli.EnvVars("NEXD_SERVICE_URL"),
				Required:   false,
				Category:   nexServiceOptions,
				Persistent: true,
			},
			&cli.BoolFlag{
				Name:       "exit-node-client",
				Usage:      "Enable this node to use an available exit node",
				Value:      false,
				Sources:    cli.EnvVars("NEXD_EXIT_NODE_CLIENT"),
				Required:   false,
				Persistent: true,
			},
			&cli.StringFlag{
				Name:       "security-group-id",
				Usage:      "Optional security group ID to use when registering used to secure this device",
				Required:   false,
				Sources:    cli.EnvVars("NEXAPI_SECURITY_GROUP_ID"),
				Persistent: true,
			},
			&cli.StringFlag{
				Name:       "reg-key",
				Usage:      "A registration key used to connect the device to the vpc",
				Sources:    cli.EnvVars("NEXD_REG_KEY"),
				Required:   false,
				Hidden:     true,
				Persistent: true,
			},
		},
		Before: func(ctx context.Context, command *cli.Command) error {
			if command.Bool("network-router") {
				if runtime.GOOS != nexodus.Linux.String() {
					return fmt.Errorf("network-router mode is only supported for Linux operating systems")
				}
				if len(command.StringSlice("advertise-cidr")) == 0 {
					return fmt.Errorf("--advertise-cidr is required for a device to be a network-router")
				}
			}
			if command.Bool("exit-node-client") {
				if runtime.GOOS != nexodus.Linux.String() {
					return fmt.Errorf("exit-node support is currently only supported for Linux operating systems")
				}
			}
			return nil
		},
		Action: func(ctx context.Context, command *cli.Command) error {
			return nexdRun(ctx, command, logger, logLevel, nexdModeAgent)
		},
	}

	app.Flags = append(app.Flags, additionalPlatformFlags...)
	sort.Slice(app.Flags, func(i, j int) bool {
		return app.Flags[i].Names()[0] < app.Flags[j].Names()[0]
	})

	if err := app.Run(context.Background(), os.Args); err != nil {
		logger.Fatal(err.Error())
	}
}
