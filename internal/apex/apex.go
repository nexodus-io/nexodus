package apex

import (
	"context"
	"fmt"
	"time"

	"github.com/redhat-et/apex/internal/messages"
	"github.com/redhat-et/apex/internal/streamer"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	readyRequestTimeout = 10
	pubSubPort          = 6379
	wgBinary            = "wg"
	wgWinBinary         = "wireguard.exe"
)

// Message Events
const (
	registerNodeRequest = "register-node-request"
)

type Apex struct {
	wireguardPubKey         string
	wireguardPvtKey         string
	wireguardPubKeyInConfig bool
	controllerIP            string
	controllerPasswd        string
	listenPort              int
	zone                    string
	requestedIP             string
	userProvidedEndpointIP  string
	localEndpointIP         string
	childPrefix             string
	stun                    bool
	hubRouter               bool
	hubRouterWgIP           string
	os                      string
	wgConfig                wgConfig
	accessToken             string
	controllerURL           string
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
	controllerURL := cCtx.Args().First()
	if controllerURL == "" {
		log.Fatal("[controller-url] required")
	}

	if err := checkOS(); err != nil {
		return nil, err
	}

	ax := &Apex{
		wireguardPubKey:        cCtx.String("public-key"),
		wireguardPvtKey:        cCtx.String("private-key"),
		controllerIP:           cCtx.String("controller"),
		controllerPasswd:       cCtx.String("controller-password"),
		listenPort:             cCtx.Int("listen-port"),
		requestedIP:            cCtx.String("request-ip"),
		userProvidedEndpointIP: cCtx.String("local-endpoint-ip"),
		childPrefix:            cCtx.String("child-prefix"),
		stun:                   cCtx.Bool("stun"),
		hubRouter:              cCtx.Bool("hub-router"),
		accessToken:            cCtx.String("with-token"),
		controllerURL:          controllerURL,
		os:                     GetOS(),
	}

	if ax.os == Windows.String() {
		if !IsCommandAvailable(wgWinBinary) {
			return nil, fmt.Errorf("wireguard.exe command not found, is wireguard installed?")
		}
	} else {
		if !IsCommandAvailable(wgBinary) {
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

	if err := ax.handleKeys(); err != nil {
		log.Fatal(err)
	}

	if ax.accessToken == "" {
		auth := NewAuthenticator(ax.controllerURL)
		if err := auth.Authenticate(ctx); err != nil {
			log.Fatal(err)
		}
		ax.accessToken, err = auth.Token()
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := RegisterDevice(ax.controllerURL, ax.wireguardPubKey, ax.accessToken); err != nil {
		log.Fatal(err)
	}

	if ax.zone, err = GetZone(ax.controllerURL, ax.accessToken); err != nil {
		log.Fatal(err)
	}

	var localEndpointIP string
	// User requested ip --request-ip takes precedent
	if ax.userProvidedEndpointIP != "" {
		localEndpointIP = ax.userProvidedEndpointIP
	}
	if ax.stun && localEndpointIP == "" {
		localEndpointIP, err = GetPubIP()
		if err != nil {
			log.Warn("Unable to determine the public facing address, falling back to the local address")
		}
	}
	if localEndpointIP == "" {
		localEndpointIP, err = ax.findLocalEndpointIp()
		if err != nil {
			log.Fatalf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %v", err)
		}
	}
	ax.localEndpointIP = localEndpointIP
	log.Infof("This node's endpoint address for building tunnels is [ %s ]", ax.localEndpointIP)

	st := streamer.NewStreamer(ctx, ax.controllerIP, pubSubPort, ax.controllerPasswd)
	defer st.Close()

	if !st.IsReady() {
		log.Fatalf("Streamer is not ready at %s", st.GetUrl())
	}
	log.Debugf("Streamer is ready and reachable")

	// ping the controller to see if it is responding via the broker, exit the agent on timeout
	if err := controllerReadyCheck(ctx, st); err != nil {
		log.Fatal(err)
	}
	log.Debugf("Controller is responding to health checks")

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

	// a hub router requires ip forwarding, OS type has already been checked
	if ax.hubRouter {
		enableForwardingIPv4()
	}

	//Agent only need to subscribe to it's own zone.
	agentZoneChan := make(chan streamer.ReceivedMessage)

	st.SubscribeAndReceive(ax.zone, agentZoneChan)
	defer st.CloseSubscription(ax.zone)

	// Agent publish the peer register request to controller channel.
	// If the zone defined is not registered with controller,
	// controller will send the error message to the peer's zone.
	err = st.PublishMessage(messages.ZoneChannelController, peerRegister)
	if err != nil {
		log.Errorf("failed to publish subscriber message: %v", err)
	}

	for {
		msg := <-agentZoneChan
		// Switch based on the streaming channel
		switch msg.Channel {
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
		default:
			log.Errorf("Message received from unknown channel [%s]", msg.Channel)
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
	if ax.hubRouter && ax.os == Darwin.String() {
		log.Fatalf("OSX nodes cannot be a hub-router, only Linux nodes")
	}
	if ax.hubRouter && ax.os == Windows.String() {
		log.Fatalf("Windows nodes cannot be a hub-router, only Linux nodes")
	}
	if ax.userProvidedEndpointIP != "" {
		if err := ValidateIp(ax.userProvidedEndpointIP); err != nil {
			log.Fatalf("the IP address passed in --local-endpoint-ip %s was not valid: %v", ax.userProvidedEndpointIP, err)
		}
	}
	if ax.requestedIP != "" {
		if err := ValidateIp(ax.requestedIP); err != nil {
			log.Fatalf("the IP address passed in --request-ip %s was not valid: %v", ax.requestedIP, err)
		}
	}
	if ax.childPrefix != "" {
		if err := ValidateCIDR(ax.childPrefix); err != nil {
			log.Fatalf("the CIDR prefix passed in --child-prefix %s was not valid: %v", ax.childPrefix, err)
		}
	}
	return nil
}

func (ax *Apex) findLocalEndpointIp() (string, error) {
	// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
	// Otherwise, discover what the public of the node is and provide that to the peers unless the --internal flag was set,
	// in which case the endpoint address will be set to an existing address on the host.
	var localEndpointIP string
	var err error
	// Darwin network discovery
	if !ax.stun && ax.os == Darwin.String() {
		localEndpointIP, err = discoverGenericIPv4(ax.controllerIP, pubSubPort)
		if err != nil {
			return "", fmt.Errorf("%v", err)
		}
	}
	// Windows network discovery
	if !ax.stun && ax.os == Windows.String() {
		localEndpointIP, err = discoverGenericIPv4(ax.controllerIP, pubSubPort)
		if err != nil {
			return "", fmt.Errorf("%v", err)
		}
	}
	// Linux network discovery
	if !ax.stun && ax.os == Linux.String() {
		linuxIP, err := discoverLinuxAddress(4)
		if err != nil {
			return "", fmt.Errorf("%v", err)
		}
		localEndpointIP = linuxIP.String()
	}
	return localEndpointIP, nil
}

// controllerReadyCheck blocks until the controller responds or the request times out
func controllerReadyCheck(ctx context.Context, st *streamer.Streamer) error {
	log.Infoln("Checking the readiness of the controller")
	healthCheckReplyChan := make(chan streamer.ReceivedMessage)
	st.SubscribeAndReceive(messages.HealthcheckReplyChannel, healthCheckReplyChan)
	if err := st.PublishMessage(messages.HealthcheckRequestChannel, messages.HealthcheckRequestMsg); err != nil {
		return err
	}

	select {
	case <-healthCheckReplyChan:
	case <-time.After(readyRequestTimeout * time.Second):
		return fmt.Errorf("controller was not reachable, ensure it is up and running")
	}
	log.Infoln("Controller is ready and reachable")
	return nil
}
