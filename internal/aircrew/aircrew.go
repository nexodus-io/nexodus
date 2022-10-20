package aircrew

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/redhat-et/jaywalking/internal/messages"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	readyRequestTimeout = 10
)

// Message Events
const (
	registerNodeRequest = "register-node-request"
)

type Aircrew struct {
	wireguardPubKey         string
	wireguardPvtKey         string
	wireguardPvtKeyFile     string
	wireguardPubKeyInConfig bool
	controllerIP            string
	controllerPasswd        string
	listenPort              int
	zone                    string
	requestedIP             string
	userProvidedEndpointIP  string
	childPrefix             string
	internalNetwork         bool
	hubRouter               bool
	os                      string
	wgConfig                wgConfig
}

type wgConfig struct {
	Interface wgLocalConfig
	Peer      []wgPeerConfig `ini:",nonunique"`
}

type wgPeerConfig struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          string
	PersistentKeepAlive string
	// AllowedIPs []string `delim:","` TODO: support an AllowedIPs slice here
}

type wgLocalConfig struct {
	PrivateKey string
	Address    string
	ListenPort int
	SaveConfig bool
	PostUp     string
	PostDown   string
}

func NewAircrew(ctx context.Context, cCtx *cli.Context) (*Aircrew, error) {

	if !IsCommandAvailable("wg") {
		return nil, fmt.Errorf("wg command not found, is wireguard installed?")
	}

	if err := checkOS(); err != nil {
		return nil, err
	}

	ac := &Aircrew{
		wireguardPubKey:        cCtx.String("public-key"),
		wireguardPvtKey:        cCtx.String("private-key"),
		wireguardPvtKeyFile:    cCtx.String("private-key-file"),
		controllerIP:           cCtx.String("controller"),
		controllerPasswd:       cCtx.String("controller-password"),
		listenPort:             cCtx.Int("listen-port"),
		zone:                   cCtx.String("zone"),
		requestedIP:            cCtx.String("request-ip"),
		userProvidedEndpointIP: cCtx.String("local-endpoint-ip"),
		childPrefix:            cCtx.String("child-prefix"),
		internalNetwork:        cCtx.Bool("internal-network"),
		hubRouter:              cCtx.Bool("hub-router"),
		os:                     GetOS(),
	}

	if err := ac.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	return ac, nil
}

func (ac *Aircrew) Run() {
	ctx := context.Background()
	var err error
	var pvtKey string

	// parse the private key for the local configuration from file or CLI
	if ac.wireguardPvtKey == "" && ac.wireguardPvtKeyFile != "" {
		pvtKey, err = ac.readPrivateKey()
		if err != nil {
			log.Fatal(err)
		}
		ac.wireguardPvtKey = pvtKey
	}

	// this conditional is last since it is expensive to do the public address lookup
	var localEndpointIP string
	if !ac.internalNetwork && ac.userProvidedEndpointIP == "" {
		localEndpointIP, err = GetPubIP()
		if err != nil {
			log.Warn("Unable to determine the public facing address")
		}
	} else {
		localEndpointIP, err = ac.findLocalEndpointIp()
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Debugf("This node's endpoint address will be [ %s ]", localEndpointIP)

	controller := fmt.Sprintf("%s:6379", ac.controllerIP)
	rc := redis.NewClient(&redis.Options{
		Addr:     controller,
		Password: ac.controllerPasswd,
	})

	// TODO: move to a redis package used by both server and agent
	_, err = rc.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Unable to connect to the redis instance at %s: %v", controller, err)
	}

	// ping the control-tower to see if it is responding via the broker, exit the agent on timeout
	if err := controlTowerReadyCheck(ctx, rc); err != nil {
		log.Fatal(err)
	}

	defer rc.Close()

	//Agent only need to subscribe to it's own zone.
	sub := rc.Subscribe(ctx, ac.zone)
	defer sub.Close()

	endpointSocket := fmt.Sprintf("%s:%d", localEndpointIP, WgListenPort)

	// Create the message describing this peer to be published
	peerRegister := messages.NewPublishPeerMessage(
		registerNodeRequest,
		ac.zone,
		ac.wireguardPubKey,
		endpointSocket,
		ac.requestedIP,
		ac.childPrefix,
		"",
		false,
		ac.hubRouter)

	// Agent publish the peer register request to controller channel.
	// If the zone defined is not registered with controltower,
	// controltower will send the error message to the peer's zone.
	err = rc.Publish(ctx, messages.ZoneChannelController, peerRegister).Err()
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
		case messages.ZoneChannelController:
			peerListing, err := messages.HandlePeerList(msg.Payload)
			if err == nil && len(peerListing) > 0 {
				// Only update the peer list if this node is a member of the zone update
				if peerListing[0].ZoneID == ac.zone {
					log.Debugf("Received message: %+v\n", peerListing)
					ac.ParseAircrewControlTowerConfig(ac.listenPort, peerListing)
					ac.DeployControlTowerWireguardConfig()
				}
			}
		case messages.ZoneChannelDefault:
			peerListing, err := messages.HandlePeerList(msg.Payload)
			if err == nil && peerListing != nil {
				log.Debugf("Received message: %+v\n", peerListing)
				ac.ParseAircrewControlTowerConfig(ac.listenPort, peerListing)
				ac.DeployControlTowerWireguardConfig()
			}
		case ac.zone:
			controlMsg, err := messages.HandleErrorMessage(msg.Payload)

			if err == nil && controlMsg.Event == messages.Error {
				log.Fatalf("Peer zone %s does not exist at control tower : [%s]:%s", ac.zone, controlMsg.Code, controlMsg.Msg)
			} else {
				peerListing, err := messages.HandlePeerList(msg.Payload)

				if err != nil {
					log.Errorf("Unsupported error message received: %v", err)
				}
				if peerListing != nil {
					log.Debugf("Received message: %+v\n", peerListing)
					ac.ParseAircrewControlTowerConfig(ac.listenPort, peerListing)
					ac.DeployControlTowerWireguardConfig()
				}
			}
		}
	}
}

func (ac *Aircrew) Shutdown(ctx context.Context) error {
	return nil
}

// Check OS and report error if the OS is not supported.
func checkOS() error {
	nodeOS := GetOS()
	switch nodeOS {
	case "windows":
		return fmt.Errorf("OS [%s] is not currently supported\n", GetOS())
	case Darwin.String():
		log.Debugf("[%s] operating system detected", nodeOS)
		// ensure the osx wireguard directory exists
		if err := CreateDirectory(WgLinuxConfPath); err != nil {
			return fmt.Errorf("Unable to create the wireguard config directory [%s]: %v", WgDarwinConfPath, err)
		}
	case Linux.String():
		log.Debugf("[%s] operating system detected", nodeOS)
		// ensure the wireguard directory exists
		if err := CreateDirectory(WgLinuxConfPath); err != nil {
			return fmt.Errorf("Unable to create the wireguard config directory [%s]: %v", WgDarwinConfPath, err)
		}
	default:
		return fmt.Errorf("OS [%s] is not supported\n", nodeOS)
	}
	return nil
}

func (ac *Aircrew) checkUnsupportedConfigs() error {
	if ac.wireguardPvtKey != "" && ac.wireguardPvtKeyFile != "" {
		return fmt.Errorf("Please use either --private-key or --private-key-file but not both")
	}
	if ac.wireguardPvtKey == "" && ac.wireguardPvtKeyFile == "" {
		return fmt.Errorf("Private key or key file location is required: use either --private-key or --private-key-file")
	}
	return nil
}

func (ac *Aircrew) readPrivateKey() (string, error) {
	// parse the private key for the local configuration from file or CLI
	if ac.wireguardPvtKeyFile != "" {
		if !FileExists(ac.wireguardPvtKeyFile) {
			return "", fmt.Errorf("private key file doesn't exist : %s", ac.wireguardPvtKeyFile)
		}
		pvtKey, err := ReadKeyFileToString(ac.wireguardPvtKeyFile)
		if err != nil {
			return "", fmt.Errorf("failed to read from private key from file %s: %v", ac.wireguardPvtKeyFile, err)
		}
		return pvtKey, nil
	}
	return "", fmt.Errorf("failed to find private key from user config and key file.")
}

func (ac *Aircrew) findLocalEndpointIp() (string, error) {
	// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
	// Otherwise, discover what the public of the node is and provide that to the peers unless the --internal flag was set,
	// in which case the endpoint address will be set to an existing address on the host.
	var localEndpointIP string
	var err error
	if ac.internalNetwork && ac.os == Darwin.String() {
		localEndpointIP, err = GetDarwinIPv4()
		if err != nil {
			return "", fmt.Errorf("unable to determine the ip address of the OSX host en0, please specify using --local-endpoint-ip: %v", err)
		}
	}
	if ac.internalNetwork && ac.os == Linux.String() {
		localEndpointIP, err = GetIPv4Linux()
		if err != nil {
			return "", fmt.Errorf("unable to determine the ip address, please specify using --local-endpoint-ip: %v", err)
		}
	}
	// User provided --local-endpoint-ip overrides --internal-network
	if ac.userProvidedEndpointIP != "" {
		if err := ValidateIp(ac.userProvidedEndpointIP); err != nil {
			return "", fmt.Errorf("the IP address passed in --local-endpoint-ip %s was not valid: %v",
				ac.userProvidedEndpointIP, err)
		}
		localEndpointIP = ac.userProvidedEndpointIP
	}
	return localEndpointIP, nil
}

// controlTowerReadyCheck blocks until the control-tower responds or the request times out
func controlTowerReadyCheck(ctx context.Context, client *redis.Client) error {
	log.Println("Checking the readiness of the control tower")
	healthCheckReplyChan := make(chan string)
	sub := client.Subscribe(ctx, messages.HealthcheckReplyChannel)
	go func() {
		for {
			output, _ := sub.ReceiveMessage(ctx)
			healthCheckReplyChan <- output.Payload
		}
	}()
	if _, err := client.Publish(ctx, messages.HealthcheckRequestChannel, messages.HealthcheckRequestMsg).Result(); err != nil {
		return err
	}
	select {
	case <-healthCheckReplyChan:
	case <-time.After(readyRequestTimeout * time.Second):
		return fmt.Errorf("control tower was not reachable, ensure it is running and attached to the broker")
	}
	log.Println("Control tower is available")
	return nil
}
