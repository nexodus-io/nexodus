package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
)

type flags struct {
	wireguardPubKey string
	configFile      string
	daemon          bool
}

var (
	cliFlags flags
)

type jaywalkState struct {
	nodePubKey        string
	jaywalkConfigFile string
	daemon            bool
	nodeOS            string
	wgConf            wgConfig
}

type wgConfig struct {
	Interface wgLocalConfig
	Peer      []wgPeerConfig `ini:",nonunique"`
}

type wgPeerConfig struct {
	PublicKey  string
	Endpoint   string
	AllowedIPs []string `delim:","`
}

type wgLocalConfig struct {
	PrivateKey string
	Address    string
	ListenPort int
	SaveConfig bool
}

type ConfigToml struct {
	Peers map[string]PeerToml `mapstructure:"Peers"`
}

// TODO: add support for AllowedIPs as a []list
type PeerToml struct {
	PublicKey   string `mapstructure:"PublicKey"`
	PrivateKey  string `mapstructure:"PrivateKey"`
	WireguardIP string `mapstructure:"AllowedIPs"`
	EndpointIP  string `mapstructure:"EndpointIP"`
}

const (
	wgListenPort     = 51871
	wgLinuxConfPath  = "/etc/wireguard/"
	wgDarwinConfPath = "/usr/local/etc/wireguard/"
	wgConfActive     = "wg0.conf"
	wgConfLatestRev  = "wg0-latest-rev.conf"
	wgIface          = "wg0"
	jaywalkConfig    = "endpoints.toml"
)

func main() {

	// instantiate the cli
	app := cli.NewApp()
	// flags are stored in the global flags variable
	app = &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "public-key",
				Value:       "",
				Usage:       "public key for the local host (required)",
				Destination: &cliFlags.wireguardPubKey,
				EnvVars:     []string{"JAYWALK_PUB_KEY"},
			},

			&cli.StringFlag{
				Name:        "config",
				Value:       "ops/endpoints.toml",
				Usage:       "configuration file",
				Destination: &cliFlags.configFile,
				EnvVars:     []string{"JAYWALK_CONFIG"},
			},
			&cli.BoolFlag{Name: "agent-mode",
				Usage:       "run as a daemon",
				Value:       false,
				Destination: &cliFlags.daemon,
				EnvVars:     []string{"JAYWALK_AGENT_MODE"},
			},
		},
	}
	app.Name = "jaywalk"
	app.Usage = "encrypted mesh networking"
	// clean up any pre-existing interfaces
	app.Before = func(c *cli.Context) error {
		if c.IsSet("clean") {
			log.Print("Cleaning up any residuals")
			// todo: implement a cleanup function
		}
		return nil
	}
	app.Action = func(c *cli.Context) error {
		// call the applications function
		runInit()
		return nil
	}
	app.Run(os.Args)
}

// runInit
func runInit() {
	if !isCommandAvailable("wg") {
		log.Fatal("wg command not found, is wireguard installed?")
	}
	// PublicKey is the unique identifier for a node and required
	if cliFlags.wireguardPubKey == "" {
		log.Fatal("the public key for this host is required to run")
	}

	var nodeOS string
	switch getOS() {
	case "windows":
		log.Fatalf("OS [%s] is not currently supported\n", getOS())
	case darwin.String():
		log.Printf("[%s] operating system detected", darwin.String())
		nodeOS = darwin.String()
	case linux.String():
		log.Printf("[%s] operating system detected", linux.String())
		nodeOS = linux.String()
	default:
		log.Fatalf("OS [%s] is not supported\n", getOS())
	}

	// check to see if the host is natted
	nat, err := isNAT(nodeOS)
	if err != nil {
		log.Printf("unable determining if the host is natted: %v", err)
	} else {
		if nat {
			log.Printf("Host appears to be natted")
		} else {
			log.Printf("Host appears to be publicly routed and not natted")
		}
	}

	js := &jaywalkState{
		nodePubKey:        cliFlags.wireguardPubKey,
		jaywalkConfigFile: cliFlags.configFile,
		daemon:            cliFlags.daemon,
		nodeOS:            nodeOS,
	}

	// parse the jaywalk config into wireguard config structs
	js.parseJaywalkConfig()
	// write the wireguard configuration to file and deploy
	js.deployWireguardConfig()

	// if agent-mode is set, run as a daemon TODO: stuff ¯\_(ツ)_/¯
	if cliFlags.daemon {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
		<-ch
	}
}
