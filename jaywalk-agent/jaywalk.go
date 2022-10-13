package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	common "github.com/redhat-et/jaywalking/jaywalk-agent/common"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

type flags struct {
	wireguardPubKey        string
	wireguardPvtKey        string
	wireguardPvtKeyFile    string
	controllerIP           string
	controllerPasswd       string
	listenPort             int
	configFile             string
	zone                   string
	requestedIP            string
	userProvidedEndpointIP string
	agentMode              bool
	childPrefix            string
}

var (
	cliFlags flags
)

type jaywalkState struct {
	nodePubKey             string
	nodePvtKey             string
	nodePubKeyInConfig     bool
	jaywalkConfigFile      string
	daemon                 bool
	nodeOS                 string
	zone                   string
	requestedIP            string
	childPrefix            string
	agentChannel           string
	userProvidedEndpointIP string
	wgConf                 wgConfig
}

type wgConfig struct {
	Interface wgLocalConfig
	Peer      []wgPeerConfig `ini:",nonunique"`
}

type wgPeerConfig struct {
	PublicKey  string
	Endpoint   string
	AllowedIPs string
	// AllowedIPs []string `delim:","` TODO: support an AllowedIPs slice here
}

type wgLocalConfig struct {
	PrivateKey string
	Address    string
	ListenPort int
	SaveConfig bool
}

type MsgEvent struct {
	Event string
	Peer  Peer
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
	wgListenPort              = 51820
	wgLinuxConfPath           = "/etc/wireguard/"
	wgDarwinConfPath          = "/usr/local/etc/wireguard/"
	wgConfActive              = "wg0.conf"
	wgConfLatestRev           = "wg0-latest-rev.conf"
	wgIface                   = "wg0"
	jaywalkConfig             = "endpoints.toml"
	zoneChannelController     = "controller"
	zoneChannelDefault        = "default"
	healthcheckRequestChannel = "supervisor-healthcheck-request"
	healthcheckReplyChannel   = "supervisor-healthcheck-reply"
	healthcheckRequestMsg     = "supervisor-ready-request"
	readyRequestTimeout       = 10
	jwLogEnv                  = "JAYWALK_LOG_LEVEL"
)

// Message Events
const (
	registerNodeRequest = "register-node-request"
)

func init() {
	// set the log level
	env := os.Getenv(jwLogEnv)
	if env == "debug" {
		log.SetLevel(log.DebugLevel)
	}
}

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
				Name:        "private-key-file",
				Value:       "",
				Usage:       "private key for the local host (recommended - but alternatively pass through --private-key",
				Destination: &cliFlags.wireguardPvtKeyFile,
				EnvVars:     []string{"JAYWALK_PRIVATE_KEY_FILE"},
			},
			&cli.StringFlag{
				Name:        "private-key",
				Value:       "",
				Usage:       "private key for the local host (demo purposes only - alternatively pass through --private-key-file",
				Destination: &cliFlags.wireguardPvtKey,
				EnvVars:     []string{"JAYWALK_PRIVATE_KEY"},
			},
			&cli.IntFlag{
				Name:        "listen-port",
				Value:       51820,
				Usage:       "port wireguard is to listen for incoming peers on",
				Destination: &cliFlags.listenPort,
				EnvVars:     []string{"JAYWALK_LISTEN_PORT"},
			},
			&cli.StringFlag{
				Name:        "controller",
				Value:       "",
				Usage:       "address of the controller (required)",
				Destination: &cliFlags.controllerIP,
				EnvVars:     []string{"JAYWALK_CONTROLLER"},
			},
			&cli.StringFlag{
				Name:        "controller-password",
				Value:       "",
				Usage:       "password for the controller (required)",
				Destination: &cliFlags.controllerPasswd,
				EnvVars:     []string{"JAYWALK_CONTROLLER_PASSWD"},
			},
			&cli.StringFlag{
				Name:        "zone",
				Value:       "default",
				Usage:       "the tenancy zone the peer is to join - zone needs to be created before joining (optional)",
				Destination: &cliFlags.zone,
				EnvVars:     []string{"JAYWALK_ZONE"},
			},
			&cli.StringFlag{
				Name:        "config",
				Value:       "",
				Usage:       "configuration file (ignored if run in agent-mode - likely deprecate)",
				Destination: &cliFlags.configFile,
				EnvVars:     []string{"JAYWALK_CONFIG"},
			},
			&cli.StringFlag{
				Name:        "request-ip",
				Value:       "",
				Usage:       "request a specific IP address from Ipam if available (optional)",
				Destination: &cliFlags.requestedIP,
				EnvVars:     []string{"JAYWALK_REQUESTED_IP"},
			},
			&cli.StringFlag{
				Name:        "local-endpoint-ip",
				Value:       "",
				Usage:       "specify the endpoint address of this node instead of being discovered (optional)",
				Destination: &cliFlags.userProvidedEndpointIP,
				EnvVars:     []string{"JAYWALK_LOCAL_ENDPOINT_IP"},
			},
			&cli.StringFlag{
				Name:        "child-prefix",
				Value:       "",
				Usage:       "request a CIDR range of addresses that will be advertised from this node (optional)",
				Destination: &cliFlags.childPrefix,
				EnvVars:     []string{"JAYWALK_REQUESTED_CHILD_PREFIX"},
			},
			&cli.BoolFlag{Name: "agent-mode",
				Usage:       "run as a agentMode",
				Value:       false,
				Destination: &cliFlags.agentMode,
				EnvVars:     []string{"JAYWALK_AGENT_MODE"},
			},
		},
	}
	app.Name = "jaywalk"
	app.Usage = "encrypted mesh networking"
	// clean up any pre-existing interfaces or processes from prior tests
	app.Before = func(c *cli.Context) error {
		if c.IsSet("clean") {
			log.Print("Cleaning up any existing benchmark interfaces")
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

func runInit() {
	if !common.IsCommandAvailable("wg") {
		log.Fatal("wg command not found, is wireguard installed?")
	}
	// PublicKey is the unique identifier for a node and required
	if cliFlags.wireguardPubKey == "" {
		log.Fatal("the public key for this host is required to run")
	}
	ctx := context.Background()

	var nodeOS string
	switch common.GetOS() {
	case "windows":
		log.Fatalf("OS [%s] is not currently supported\n", common.GetOS())
	case common.Darwin.String():
		log.Printf("[%s] operating system detected", common.Darwin.String())
		nodeOS = common.Darwin.String()
		// ensure the osx wireguard directory exists
		if err := common.CreateDirectory(wgLinuxConfPath); err != nil {
			log.Fatalf("Unable to create the wireguard config directory [%s]: %v", wgDarwinConfPath, err)
		}
	case common.Linux.String():
		log.Printf("[%s] operating system detected", common.Linux.String())
		nodeOS = common.Linux.String()
		// ensure the wireguard directory exists
		if err := common.CreateDirectory(wgLinuxConfPath); err != nil {
			log.Fatalf("Unable to create the wireguard config directory [%s]: %v", wgDarwinConfPath, err)
		}
	default:
		log.Fatalf("OS [%s] is not supported\n", common.GetOS())
	}

	// check to see if the host is natted
	nat, err := common.IsNAT(nodeOS)
	if err != nil {
		log.Printf("unable determining if the host is natted: %v", err)
	} else {
		if nat {
			log.Printf("Host appears to be natted")
		} else {
			log.Printf("Host appears to be publicly routed and not natted")
		}
	}

	// parse the private key for the local configuration from file or CLI
	var pvtKey string
	if cliFlags.wireguardPvtKey != "" && cliFlags.wireguardPvtKeyFile != "" {
		log.Fatalf("Please use either --private-key or --private-key-file but not both")
	}
	if cliFlags.wireguardPvtKey == "" && cliFlags.wireguardPvtKeyFile == "" {
		log.Fatalf("Private key or key file location is required: use either --private-key or --private-key-file")
	}
	if cliFlags.wireguardPvtKey != "" {
		pvtKey = cliFlags.wireguardPvtKey
	}
	if cliFlags.wireguardPvtKeyFile != "" {
		if !common.FileExists(cliFlags.wireguardPvtKeyFile) {
			log.Fatalf("Failed to retrieve the private key from file: %s", cliFlags.wireguardPvtKeyFile)
		}
		pvtKey, err = common.ReadKeyFileToString(cliFlags.wireguardPvtKeyFile)
		if err != nil {
			log.Fatalf("Failed to retrieve the private key from file %s: %v", cliFlags.wireguardPvtKeyFile, err)
		}
	}

	js := &jaywalkState{
		nodePubKey:             cliFlags.wireguardPubKey,
		nodePvtKey:             pvtKey,
		jaywalkConfigFile:      cliFlags.configFile,
		daemon:                 cliFlags.agentMode,
		nodeOS:                 nodeOS,
		zone:                   cliFlags.zone,
		requestedIP:            cliFlags.requestedIP,
		childPrefix:            cliFlags.childPrefix,
		userProvidedEndpointIP: cliFlags.userProvidedEndpointIP,
	}

	if !cliFlags.agentMode {
		// parse the jaywalk config into wireguard config structs
		js.parseJaywalkConfig()
		// write the wireguard configuration to file and deploy
		js.deployWireguardConfig()
	}

	// run as a persistent agent
	if cliFlags.agentMode {
		controller := fmt.Sprintf("%s:6379", cliFlags.controllerIP)
		rc := redis.NewClient(&redis.Options{
			Addr:     controller,
			Password: cliFlags.controllerPasswd,
		})

		// TODO: move to a redis package used by both server and agent
		_, err := rc.Ping(ctx).Result()
		if err != nil {
			log.Fatalf("Unable to connect to the redis instance at %s: %v", controller, err)
		}

		// ping the supervisor to see if it is responding via the broker, exit the agent on timeout
		superVisorReadyCheck(ctx, rc)

		defer rc.Close()

		subChannel := zoneChannelDefault
		if js.zone != zoneChannelDefault {
			subChannel = zoneChannelController
		}

		sub := rc.Subscribe(ctx, subChannel)
		defer sub.Close()

		// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
		// Otherwise, discover what the public of the node is and provide that to the peers.
		var localEndpointIP string
		if js.userProvidedEndpointIP != "" {
			if err := common.ValidateIp(js.userProvidedEndpointIP); err != nil {
				log.Fatalf("the IP address passed in --local-endpoint-ip %s was not valid: %v",
					js.userProvidedEndpointIP, err)
			}
			localEndpointIP = js.userProvidedEndpointIP
		} else {
			localEndpointIP, err = common.GetPubIP()
			if err != nil {
				log.Warn("Unable to determine the public facing address")
			}
		}
		endpointSocket := fmt.Sprintf("%s:%d", localEndpointIP, wgListenPort)
		// Create the message describing this peer to be published
		peerRegister := publishMessage(
			registerNodeRequest,
			js.zone,
			js.nodePubKey,
			endpointSocket,
			js.requestedIP,
			js.childPrefix)

		// Agent only needs to subscribe
		if js.zone == "default" {
			js.agentChannel = zoneChannelDefault
		} else {
			js.agentChannel = zoneChannelController
		}

		err = rc.Publish(ctx, js.agentChannel, peerRegister).Err()
		if err != nil {
			log.Errorf("failed to publish subscriber message: %v", err)
		}
		defer rc.Close()
		for {
			msg, err := sub.ReceiveMessage(ctx)
			if err != nil {
				log.Fatalf("Failed to subscribe to the controller: %v", err)
				os.Exit(1)
			}
			// Switch based on the streaming channel
			switch msg.Channel {
			case zoneChannelController:
				peerListing := handleMsg(msg.Payload)
				if len(peerListing) > 0 {
					// Only update the peer list if this node is a member of the zone update
					if peerListing[0].Zone == js.zone {
						log.Debugf("Received message: %+v\n", peerListing)
						js.parseJaywalkSupervisorConfig(peerListing)
						js.deploySupervisorWireguardConfig()
					}
				}
			case zoneChannelDefault:
				peerListing := handleMsg(msg.Payload)
				if peerListing != nil {
					log.Debugf("Received message: %+v\n", peerListing)
					js.parseJaywalkSupervisorConfig(peerListing)
					js.deploySupervisorWireguardConfig()
				}
			}
		}
	}
}

func publishMessage(event, zone, pubKey, endpointIP, requestedIP, childPrefix string) string {
	msg := MsgEvent{}
	msg.Event = event
	peer := Peer{
		PublicKey:   pubKey,
		EndpointIP:  endpointIP,
		Zone:        zone,
		NodeAddress: requestedIP,
		ChildPrefix: childPrefix,
	}
	msg.Peer = peer
	jMsg, _ := json.Marshal(&msg)
	return string(jMsg)
}

// superVisorReadyCheck blocks until the supervisor responds or the request times out
func superVisorReadyCheck(ctx context.Context, client *redis.Client) {
	log.Println("Checking the readiness of the supervisor")
	healthCheckReplyChan := make(chan string)
	sub := client.Subscribe(ctx, healthcheckReplyChannel)
	go func() {
		for {
			output, _ := sub.ReceiveMessage(ctx)
			healthCheckReplyChan <- output.Payload
		}
	}()
	client.Publish(ctx, healthcheckRequestChannel, healthcheckRequestMsg).Result()
	select {
	case <-healthCheckReplyChan:
	case <-time.After(readyRequestTimeout * time.Second):
		log.Fatal("Supervisor was not reachable, ensure it is running and attached to the broker")
	}
	log.Println("Supervisor is available")
}
