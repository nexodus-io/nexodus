package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/redhat-et/jaywalking/internal/aircrew"
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
	childPrefix            string
	agentMode              bool
	internalNetwork        bool
}

var (
	cliFlags flags
)

type MsgEvent struct {
	Event string
	Peer  aircrew.Peer
}

const (
	zoneChannelController     = "controller"
	zoneChannelDefault        = "default"
	healthcheckRequestChannel = "controltower-healthcheck-request"
	healthcheckReplyChannel   = "controltower-healthcheck-reply"
	healthcheckRequestMsg     = "controltower-ready-request"
	readyRequestTimeout       = 10

	aircrewLogEnv = "AIRCREW_LOG_LEVEL"
)

// Message Events
const (
	registerNodeRequest = "register-node-request"
)

func init() {
	// set the log level
	env := os.Getenv(aircrewLogEnv)
	if env == "debug" {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	// flags are stored in the global flags variable
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "public-key",
				Value:       "",
				Usage:       "public key for the local host (required)",
				Destination: &cliFlags.wireguardPubKey,
				EnvVars:     []string{"AIRCREW_PUB_KEY"},
			},
			&cli.StringFlag{
				Name:        "private-key-file",
				Value:       "",
				Usage:       "private key for the local host (recommended - but alternatively pass through --private-key",
				Destination: &cliFlags.wireguardPvtKeyFile,
				EnvVars:     []string{"AIRCREW_PRIVATE_KEY_FILE"},
			},
			&cli.StringFlag{
				Name:        "private-key",
				Value:       "",
				Usage:       "private key for the local host (demo purposes only - alternatively pass through --private-key-file",
				Destination: &cliFlags.wireguardPvtKey,
				EnvVars:     []string{"AIRCREW_PRIVATE_KEY"},
			},
			&cli.IntFlag{
				Name:        "listen-port",
				Value:       51820,
				Usage:       "port wireguard is to listen for incoming peers on",
				Destination: &cliFlags.listenPort,
				EnvVars:     []string{"AIRCREW_LISTEN_PORT"},
			},
			&cli.StringFlag{
				Name:        "controller",
				Value:       "",
				Usage:       "address of the controller (required)",
				Destination: &cliFlags.controllerIP,
				EnvVars:     []string{"AIRCREW_CONTROLLER"},
			},
			&cli.StringFlag{
				Name:        "controller-password",
				Value:       "",
				Usage:       "password for the controller (required)",
				Destination: &cliFlags.controllerPasswd,
				EnvVars:     []string{"AIRCREW_CONTROLLER_PASSWD"},
			},
			&cli.StringFlag{
				Name:        "zone",
				Value:       "default",
				Usage:       "the tenancy zone the peer is to join - zone needs to be created before joining (optional)",
				Destination: &cliFlags.zone,
				EnvVars:     []string{"AIRCREW_ZONE"},
			},
			&cli.StringFlag{
				Name:        "config",
				Value:       "",
				Usage:       "configuration file (ignored if run in agent-mode - likely deprecate)",
				Destination: &cliFlags.configFile,
				EnvVars:     []string{"AIRCREW_CONFIG"},
			},
			&cli.StringFlag{
				Name:        "request-ip",
				Value:       "",
				Usage:       "request a specific IP address from Ipam if available (optional)",
				Destination: &cliFlags.requestedIP,
				EnvVars:     []string{"AIRCREW_REQUESTED_IP"},
			},
			&cli.StringFlag{
				Name:        "local-endpoint-ip",
				Value:       "",
				Usage:       "specify the endpoint address of this node instead of being discovered (optional)",
				Destination: &cliFlags.userProvidedEndpointIP,
				EnvVars:     []string{"AIRCREW_LOCAL_ENDPOINT_IP"},
			},
			&cli.StringFlag{
				Name:        "child-prefix",
				Value:       "",
				Usage:       "request a CIDR range of addresses that will be advertised from this node (optional)",
				Destination: &cliFlags.childPrefix,
				EnvVars:     []string{"AIRCREW_REQUESTED_CHILD_PREFIX"},
			},
			&cli.BoolFlag{Name: "agent-mode",
				Usage:       "run as an agent",
				Value:       true,
				Destination: &cliFlags.agentMode,
				EnvVars:     []string{"AIRCREW_AGENT_MODE"},
			},
			&cli.BoolFlag{Name: "internal-network",
				Usage:       "do not discover the public address for this host, use the local address for peering",
				Value:       false,
				Destination: &cliFlags.internalNetwork,
				EnvVars:     []string{"AIRCREW_INTERNAL_NETWORK"},
			},
		},
	}
	app.Name = "aircrew"
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
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func runInit() {
	if !aircrew.IsCommandAvailable("wg") {
		log.Fatal("wg command not found, is wireguard installed?")
	}
	// PublicKey is the unique identifier for a node and required
	if cliFlags.wireguardPubKey == "" {
		log.Fatal("the public key for this host is required to run")
	}
	ctx := context.Background()
	var err error
	var nodeOS string
	switch aircrew.GetOS() {
	case "windows":
		log.Fatalf("OS [%s] is not currently supported\n", aircrew.GetOS())
	case aircrew.Darwin.String():
		log.Printf("[%s] operating system detected", aircrew.Darwin.String())
		nodeOS = aircrew.Darwin.String()
		// ensure the osx wireguard directory exists
		if err := aircrew.CreateDirectory(aircrew.WgLinuxConfPath); err != nil {
			log.Fatalf("Unable to create the wireguard config directory [%s]: %v", aircrew.WgDarwinConfPath, err)
		}
	case aircrew.Linux.String():
		log.Printf("[%s] operating system detected", aircrew.Linux.String())
		nodeOS = aircrew.Linux.String()
		// ensure the wireguard directory exists
		if err := aircrew.CreateDirectory(aircrew.WgLinuxConfPath); err != nil {
			log.Fatalf("Unable to create the wireguard config directory [%s]: %v", aircrew.WgDarwinConfPath, err)
		}
	default:
		log.Fatalf("OS [%s] is not supported\n", aircrew.GetOS())
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
		if !aircrew.FileExists(cliFlags.wireguardPvtKeyFile) {
			log.Fatalf("Failed to retrieve the private key from file: %s", cliFlags.wireguardPvtKeyFile)
		}
		pvtKey, err = aircrew.ReadKeyFileToString(cliFlags.wireguardPvtKeyFile)
		if err != nil {
			log.Fatalf("Failed to retrieve the private key from file %s: %v", cliFlags.wireguardPvtKeyFile, err)
		}
	}

	// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
	// Otherwise, discover what the public of the node is and provide that to the peers unless the --internal flag was set,
	// in which case the endpoint address will be set to an existing address on the host.
	var localEndpointIP string
	if cliFlags.internalNetwork && nodeOS == aircrew.Darwin.String() {
		localEndpointIP, err = aircrew.GetDarwinIPv4()
		if err != nil {
			log.Fatalf("unable to determine the ip address of the OSX host en0, please specify using --local-endpoint-ip: %v", err)
		}
	}
	if cliFlags.internalNetwork && nodeOS == aircrew.Linux.String() {
		localEndpointIP, err = aircrew.GetIPv4Linux()
		if err != nil {
			log.Fatalf("unable to determine the ip address, please specify using --local-endpoint-ip")
		}
	}
	// User provided --local-endpoint-ip overrides --internal-network
	if cliFlags.userProvidedEndpointIP != "" {
		if err := aircrew.ValidateIp(cliFlags.userProvidedEndpointIP); err != nil {
			log.Fatalf("the IP address passed in --local-endpoint-ip %s was not valid: %v",
				cliFlags.userProvidedEndpointIP, err)
		}
		localEndpointIP = cliFlags.userProvidedEndpointIP
	}
	// this conditional is last since it is expensive to do the public address lookup
	if !cliFlags.internalNetwork && cliFlags.userProvidedEndpointIP == "" {
		localEndpointIP, err = aircrew.GetPubIP()
		if err != nil {
			log.Warn("Unable to determine the public facing address")
		}
	}
	log.Debugf("This node's endpoint address will be [ %s ]", localEndpointIP)

	as := &aircrew.AircrewState{
		NodePubKey:        cliFlags.wireguardPubKey,
		NodePvtKey:        pvtKey,
		AircrewConfigFile: cliFlags.configFile,
		Daemon:            cliFlags.agentMode,
		NodeOS:            nodeOS,
		Zone:              cliFlags.zone,
		RequestedIP:       cliFlags.requestedIP,
		ChildPrefix:       cliFlags.childPrefix,
		UserEndpointIP:    cliFlags.userProvidedEndpointIP,
	}

	if !cliFlags.agentMode {
		// parse the Aircrew config into wireguard config structs
		as.ParseAircrewConfig()
		// write the wireguard configuration to file and deploy
		as.DeployWireguardConfig()
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

		// ping the control-tower to see if it is responding via the broker, exit the agent on timeout
		if err := controlTowerReadyCheck(ctx, rc); err != nil {
			log.Fatal(err)
		}

		defer rc.Close()

		subChannel := zoneChannelDefault
		if as.Zone != zoneChannelDefault {
			subChannel = zoneChannelController
		}

		sub := rc.Subscribe(ctx, subChannel)
		defer sub.Close()

		endpointSocket := fmt.Sprintf("%s:%d", localEndpointIP, aircrew.WgListenPort)
		// Create the message describing this peer to be published
		peerRegister := publishMessage(
			registerNodeRequest,
			as.Zone,
			as.NodePubKey,
			endpointSocket,
			as.RequestedIP,
			as.ChildPrefix)

		// Agent only needs to subscribe
		if as.Zone == "default" {
			as.AgentChannel = zoneChannelDefault
		} else {
			as.AgentChannel = zoneChannelController
		}

		err = rc.Publish(ctx, as.AgentChannel, peerRegister).Err()
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
				peerListing := aircrew.HandleMsg(msg.Payload)
				if len(peerListing) > 0 {
					// Only update the peer list if this node is a member of the zone update
					if peerListing[0].Zone == as.Zone {
						log.Debugf("Received message: %+v\n", peerListing)
						as.ParseAircrewControlTowerConfig(cliFlags.listenPort, peerListing)
						as.DeployControlTowerWireguardConfig()
					}
				}
			case zoneChannelDefault:
				peerListing := aircrew.HandleMsg(msg.Payload)
				if peerListing != nil {
					log.Debugf("Received message: %+v\n", peerListing)
					as.ParseAircrewControlTowerConfig(cliFlags.listenPort, peerListing)
					as.DeployControlTowerWireguardConfig()
				}
			}
		}
	}
}

func publishMessage(event, zone, pubKey, endpointIP, requestedIP, childPrefix string) string {
	msg := MsgEvent{}
	msg.Event = event
	peer := aircrew.Peer{
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

// controlTowerReadyCheck blocks until the control-tower responds or the request times out
func controlTowerReadyCheck(ctx context.Context, client *redis.Client) error {
	log.Println("Checking the readiness of the control tower")
	healthCheckReplyChan := make(chan string)
	sub := client.Subscribe(ctx, healthcheckReplyChannel)
	go func() {
		for {
			output, _ := sub.ReceiveMessage(ctx)
			healthCheckReplyChan <- output.Payload
		}
	}()
	if _, err := client.Publish(ctx, healthcheckRequestChannel, healthcheckRequestMsg).Result(); err != nil {
		return err
	}
	select {
	case <-healthCheckReplyChan:
	case <-time.After(readyRequestTimeout * time.Second):
		return fmt.Errorf("Control tower was not reachable, ensure it is running and attached to the broker")
	}
	log.Println("Control tower is available")
	return nil
}
