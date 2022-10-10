package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

type flags struct {
	wireguardPubKey        string
	wireguardPvtKey        string
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
	nodePubKeyInConfig     bool
	jaywalkConfigFile      string
	daemon                 bool
	nodeOS                 string
	zone                   string
	requestedIP            string
	childPrefix            string
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
	zoneChannelBlue           = "zone-blue"
	zoneChannelRed            = "zone-red"
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
				Name:        "private-key",
				Value:       "",
				Usage:       "private key for the local host (required)",
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
				Value:       "zone-blue",
				Usage:       "the tenancy zone the peer is to join (optional)",
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
	if !isCommandAvailable("wg") {
		log.Fatal("wg command not found, is wireguard installed?")
	}
	// PublicKey is the unique identifier for a node and required
	if cliFlags.wireguardPubKey == "" {
		log.Fatal("the public key for this host is required to run")
	}
	ctx := context.Background()

	var nodeOS string
	switch getOS() {
	case "windows":
		log.Fatalf("OS [%s] is not currently supported\n", getOS())
	case darwin.String():
		log.Printf("[%s] operating system detected", darwin.String())
		nodeOS = darwin.String()
		// ensure the osx wireguard directory exists
		if err := createDirectory(wgLinuxConfPath); err != nil {
			log.Fatalf("Unable to create the wireguard config directory [%s]: %v", wgDarwinConfPath, err)
		}
	case linux.String():
		log.Printf("[%s] operating system detected", linux.String())
		nodeOS = linux.String()
		// ensure the wireguard directory exists
		if err := createDirectory(wgLinuxConfPath); err != nil {
			log.Fatalf("Unable to create the wireguard config directory [%s]: %v", wgDarwinConfPath, err)
		}
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
		nodePubKey:             cliFlags.wireguardPubKey,
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
		defer rc.Close()

		// ping the supervisor to see if it is responding via the broker, exit the agent on timeout
		superVisorReadyCheck(ctx, rc)

		pubsub := rc.Subscribe(ctx, js.zone)
		defer pubsub.Close()

		// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
		// Otherwise, discover what the public of the node is and provide that to the peers.
		var localEndpointIP string
		if js.userProvidedEndpointIP != "" {
			if err := validateIp(js.userProvidedEndpointIP); err != nil {
				log.Fatal("the IP address passed in --local-endpoint-ip %s was not valid: %v",
					js.userProvidedEndpointIP, err)
			}
			localEndpointIP = js.userProvidedEndpointIP
		} else {
			localEndpointIP, err = getPubIP()
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

		err = rc.Publish(ctx, js.zone, peerRegister).Err()
		if err != nil {
			log.Errorf("failed to publish subscriber message: %v", err)
		}
		log.Printf("Publishing registration to supervisor: %v\n", peerRegister)

		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				log.Fatalf("Failed to subscribe to the controller: %v", err)
				os.Exit(1)
			}
			// Switch based on the streaming channel
			switch msg.Channel {
			case zoneChannelBlue:
				peerListing := handleMsg(msg.Payload)
				if peerListing != nil {
					log.Debugf("Received message: %+v\n", peerListing)
					js.parseJaywalkSupervisorConfig(peerListing)
					js.deploySupervisorWireguardConfig()
				}
			case zoneChannelRed:
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
