package apex

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/redhat-et/apex/internal/messages"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	readyRequestTimeout = 10
	pubSubPort          = 6379
)

// Message Events
const (
	registerNodeRequest = "register-node-request"
)

type Apex struct {
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

func NewApex(ctx context.Context, cCtx *cli.Context) (*Apex, error) {

	if err := checkOS(); err != nil {
		return nil, err
	}

	ax := &Apex{
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

	if ax.os == Windows.String() {
		if !IsCommandAvailable("wireguard.exe") {
			return nil, fmt.Errorf("wireguard.exe command not found, is wireguard installed?")
		}
	} else {
		if !IsCommandAvailable("wg") {
			return nil, fmt.Errorf("wg command not found, is wireguard installed?")
		}
	}

	if err := ax.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	return ax, nil
}

func (ax *Apex) Run() {
	ctx := context.Background()
	var err error
	var pvtKey string

	// parse the private key for the local configuration from file or CLI
	if ax.wireguardPvtKey == "" && ax.wireguardPvtKeyFile != "" {
		pvtKey, err = ax.readPrivateKey()
		if err != nil {
			log.Fatal(err)
		}
		ax.wireguardPvtKey = pvtKey
	}

	// this conditional is last since it is expensive to do the public address lookup
	var localEndpointIP string
	if !ax.internalNetwork && ax.userProvidedEndpointIP == "" {
		localEndpointIP, err = GetPubIP()
		if err != nil {
			log.Warn("Unable to determine the public facing address")
		}
	} else {
		localEndpointIP, err = ax.findLocalEndpointIp()
		if err != nil {
			log.Fatalf("unable to determine the ip address of the OSX host en0, please specify using --local-endpoint-ip: %v", err)
		}
	}
	log.Infof("This node's endpoint address for building tunnels is [ %s ]", localEndpointIP)

	controller := fmt.Sprintf("%s:%d", ax.controllerIP, pubSubPort)
	rc := redis.NewClient(&redis.Options{
		Addr:     controller,
		Password: ax.controllerPasswd,
	})

	// TODO: move to a redis package used by both server and agent
	_, err = rc.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Unable to connect to the redis instance at %s: %v", controller, err)
	}
	log.Debugf("Pubsub is reachable")

	// ping the controller to see if it is responding via the broker, exit the agent on timeout
	if err := controllerReadyCheck(ctx, rc); err != nil {
		log.Fatal(err)
	}
	defer rc.Close()
	log.Debugf("Controller is responding to health checks")

	//Agent only need to subscribe to it's own zone.
	sub := rc.Subscribe(ctx, ax.zone)
	defer sub.Close()

	endpointSocket := fmt.Sprintf("%s:%d", localEndpointIP, WgListenPort)

	// Create the message describing this peer to be published
	peerRegister := messages.NewPublishPeerMessage(
		registerNodeRequest,
		ax.zone,
		ax.wireguardPubKey,
		endpointSocket,
		ax.requestedIP,
		ax.childPrefix,
		"",
		false,
		ax.hubRouter)

	// Agent publish the peer register request to controller channel.
	// If the zone defined is not registered with controller,
	// controller will send the error message to the peer's zone.
	err = rc.Publish(ctx, messages.ZoneChannelController, peerRegister).Err()
	if err != nil {
		log.Errorf("failed to publish subscriber message: %v", err)
	}
	defer rc.Close()
	for {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			log.Fatalf("Failed to subscribe to the controller channel: %v", err)
			os.Exit(1)
		}
		// Switch based on the streaming channel
		switch msg.Channel {
		case messages.ZoneChannelController:
			peerListing, err := messages.HandlePeerList(msg.Payload)
			if err == nil && len(peerListing) > 0 {
				// Only update the peer list if this node is a member of the zone update
				if peerListing[0].ZoneID == ax.zone {
					log.Debugf("Received message: %+v\n", peerListing)
					ax.ParseWireguardConfig(ax.listenPort, peerListing)
					ax.DeployWireguardConfig()
				}
			}
		case messages.ZoneChannelDefault:
			peerListing, err := messages.HandlePeerList(msg.Payload)
			if err == nil && peerListing != nil {
				log.Debugf("Received message: %+v\n", peerListing)
				ax.ParseWireguardConfig(ax.listenPort, peerListing)
				ax.DeployWireguardConfig()
			}
		case ax.zone:
			controlMsg, err := messages.HandleErrorMessage(msg.Payload)

			if err == nil && controlMsg.Event == messages.Error {
				log.Fatalf("Peer zone %s does not exist at controller : [%s]:%s", ax.zone, controlMsg.Code, controlMsg.Msg)
			} else {
				peerListing, err := messages.HandlePeerList(msg.Payload)

				if err != nil {
					log.Errorf("Unsupported error message received: %v", err)
				}
				if peerListing != nil {
					log.Debugf("Received message: %+v\n", peerListing)
					ax.ParseWireguardConfig(ax.listenPort, peerListing)
					ax.DeployWireguardConfig()
				}
			}
		}
	}
}

func (ax *Apex) Shutdown(ctx context.Context) error {
	return nil
}

// Check OS and report error if the OS is not supported.
func checkOS() error {
	nodeOS := GetOS()
	switch nodeOS {
	case Darwin.String():
		log.Debugf("[%s] operating system detected", nodeOS)
		// ensure the osx wireguard directory exists
		if err := CreateDirectory(WgDarwinConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %v", WgDarwinConfPath, err)
		}
	case Windows.String():
		log.Debugf("[%s] operating system detected", nodeOS)
		// ensure the windows wireguard directory exists
		if err := CreateDirectory(WgWindowsConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %v", WgWindowsConfPath, err)
		}
	case Linux.String():
		log.Debugf("[%s] operating system detected", nodeOS)
		// ensure the linux wireguard directory exists
		if err := CreateDirectory(WgLinuxConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %v", WgLinuxConfPath, err)
		}
	default:
		return fmt.Errorf("OS [%s] is not supported\n", nodeOS)
	}
	return nil
}

// checkUnsupportedConfigs general matrix checks of required information or constraints to run the agent and join the mesh
func (ax *Apex) checkUnsupportedConfigs() error {
	if ax.wireguardPvtKey != "" && ax.wireguardPvtKeyFile != "" {
		return fmt.Errorf("please use either --private-key or --private-key-file but not both")
	}
	if ax.wireguardPvtKey == "" && ax.wireguardPvtKeyFile == "" {
		return fmt.Errorf("private key or key file location is required: use either --private-key or --private-key-file")
	}
	if ax.hubRouter && ax.os == Darwin.String() {
		log.Fatalf("OSX nodes cannot be a hub-router, only Linux nodes")
	}
	if ax.hubRouter && ax.os == Windows.String() {
		log.Fatalf("Windows nodes cannot be a hub-router, only Linux nodes")
	}
	return nil
}

// readPrivateKey parses the private key for the local configuration from file or CLI
func (ax *Apex) readPrivateKey() (string, error) {
	if ax.wireguardPvtKeyFile != "" {
		if !FileExists(ax.wireguardPvtKeyFile) {
			return "", fmt.Errorf("private key file doesn't exist : %s", ax.wireguardPvtKeyFile)
		}
		pvtKey, err := ReadKeyFileToString(ax.wireguardPvtKeyFile)
		if err != nil {
			return "", fmt.Errorf("failed to read from private key from file %s: %v", ax.wireguardPvtKeyFile, err)
		}
		return pvtKey, nil
	}
	return "", fmt.Errorf("failed to find private key from user config and key file.")
}

func (ax *Apex) findLocalEndpointIp() (string, error) {
	// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
	// Otherwise, discover what the public of the node is and provide that to the peers unless the --internal flag was set,
	// in which case the endpoint address will be set to an existing address on the host.
	var localEndpointIP string
	var err error
	// Darwin network discovery
	if ax.internalNetwork && ax.os == Darwin.String() {
		localEndpointIP, err = discoverGenericIPv4(ax.controllerIP, pubSubPort)
		if err != nil {
			return "", fmt.Errorf("unable to determine the ip address of the OSX host en0, please specify using --local-endpoint-ip: %v", err)
		}
	}
	// Windows network discovery
	if ax.internalNetwork && ax.os == Windows.String() {
		localEndpointIP, err = discoverGenericIPv4(ax.controllerIP, pubSubPort)
		if err != nil {
			return "", fmt.Errorf("unable to determine the ip address of the OSX host en0, please specify using --local-endpoint-ip: %v", err)
		}
	}
	// Linux network discovery
	if ax.internalNetwork && ax.os == Linux.String() {
		linuxIP, err := discoverLinuxAddress(4)
		if err != nil {
			return "", fmt.Errorf("unable to determine the Linux node ip address, please specify the address using --local-endpoint-ip: %v", err)
		}
		localEndpointIP = linuxIP.String()
	}
	// User provided --local-endpoint-ip overrides --internal-network
	if ax.userProvidedEndpointIP != "" {
		if err := ValidateIp(ax.userProvidedEndpointIP); err != nil {
			return "", fmt.Errorf("the IP address passed in --local-endpoint-ip %s was not valid: %v",
				ax.userProvidedEndpointIP, err)
		}
		localEndpointIP = ax.userProvidedEndpointIP
	}
	return localEndpointIP, nil
}

// controllerReadyCheck blocks until the controller responds or the request times out
func controllerReadyCheck(ctx context.Context, client *redis.Client) error {
	log.Println("checking the readiness of the controller")
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
		return fmt.Errorf("controller was not reachable, ensure it is up and running")
	}
	log.Println("controller is available")
	return nil
}
