package apex

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/client"
	"github.com/redhat-et/apex/internal/models"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	pollInterval = 5 * time.Second
	wgBinary     = "wg"
	wgGoBinary   = "wireguard-go"
	wgWinBinary  = "wireguard.exe"
	REGISTER_URL = "/api/zones/%s/peers"
	DEVICE_URL   = "/api/devices/%s"
)

type Apex struct {
	wireguardPubKey         string
	wireguardPvtKey         string
	wireguardPubKeyInConfig bool
	controllerIP            string
	controllerPasswd        string
	listenPort              int
	zone                    uuid.UUID
	requestedIP             string
	userProvidedEndpointIP  string
	localEndpointIP         string
	childPrefix             string
	stun                    bool
	hubRouter               bool
	hubRouterWgIP           string
	os                      string
	wgConfig                wgConfig
	client                  client.Client
	controllerURL           *url.URL
	// caches peers by their UUID
	peerCache map[uuid.UUID]models.Peer
	// maps device_ids to public keys
	keyCache             map[uuid.UUID]string
	wgLocalAddress       string
	nodeReflexiveAddress string
}

type wgConfig struct {
	Interface wgLocalConfig
	Peer      []wgPeerConfig `ini:",nonunique"`
}

type wgPeerConfig struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepAlive string
	// AllowedIPs []string `delim:","` TODO: support an AllowedIPs slice here
}

type wgLocalConfig struct {
	PrivateKey string
	ListenPort int
}

func NewApex(ctx context.Context, cCtx *cli.Context) (*Apex, error) {
	if err := binaryChecks(); err != nil {
		return nil, err
	}

	controller := cCtx.Args().First()
	if controller == "" {
		log.Fatal("<controller-url> required")
	}

	controllerURL, err := url.Parse(controller)
	if err != nil {
		log.Fatalf("error: <controller-url> is not a valid URL: %s", err)
	}

	withToken := cCtx.String("with-token")
	var auth client.Authenticator
	if withToken == "" {
		var err error
		auth, err = client.NewDeviceFlowAuthenticator(ctx, controllerURL)
		if err != nil {
			log.Fatalf("authentication error: %+v", err)
		}
	} else {
		auth = client.NewTokenAuthenticator(withToken)
	}

	client, err := client.NewClient(controller, auth)
	if err != nil {
		log.Fatalf("error creating client: %+v", err)
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
		client:                 client,
		os:                     GetOS(),
		peerCache:              make(map[uuid.UUID]models.Peer),
		keyCache:               make(map[uuid.UUID]string),
		controllerURL:          controllerURL,
	}

	if err := ax.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	ax.nodePrep()

	return ax, nil
}

func (ax *Apex) Run() {
	var err error

	if err := ax.handleKeys(); err != nil {
		log.Fatalf("handleKeys: %+v", err)
	}

	device, err := ax.client.CreateDevice(ax.wireguardPubKey)
	if err != nil {
		log.Fatalf("device register error: %+v", err)
	}
	log.Infof("Device Registered with UUID: %s", device.ID)

	user, err := ax.client.GetCurrentUser()
	if err != nil {
		log.Fatalf("get zone error: %+v", err)
	}
	log.Infof("Device belongs in zone: %s", user.ZoneID)
	ax.zone = user.ZoneID

	var localEndpointIP string
	// User requested ip --request-ip takes precedent
	if ax.userProvidedEndpointIP != "" {
		localEndpointIP = ax.userProvidedEndpointIP
	}
	if ax.stun && localEndpointIP == "" {
		localEndpointIP, err = GetPubIPv4()
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

	endpointSocket := fmt.Sprintf("%s:%d", localEndpointIP, WgListenPort)

	_, err = ax.client.CreatePeerInZone(
		user.ZoneID,
		device.ID,
		endpointSocket,
		ax.requestedIP,
		ax.childPrefix,
		ax.hubRouter,
		false,
		"",
		ax.nodeReflexiveAddress,
	)
	if err != nil {
		log.Fatalf("error creating peer: %+v", err)
	}
	log.Info("Successfully registered with Apex Controller")

	// a hub router requires ip forwarding and iptables rules, OS type has already been checked
	if ax.hubRouter {
		enableForwardingIPv4()
		hubRouterIpTables()
	}

	if err := ax.Reconcile(ax.zone); err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(pollInterval)
	for range ticker.C {
		if err := ax.Reconcile(user.ZoneID); err != nil {
			// TODO: Add smarter reconciliation logic
			// to handle disconnects and/or timeouts etc...
			log.Fatal(err)
		}
	}
}

func (ax *Apex) Reconcile(zoneID uuid.UUID) error {
	peerListing, err := ax.client.GetZonePeers(zoneID)
	if err != nil {
		return err
	}

	changed := false
	for _, p := range peerListing {
		existing, ok := ax.peerCache[p.ID]
		if !ok {
			changed = true
			ax.peerCache[p.ID] = p
		}
		if !reflect.DeepEqual(existing, p) {
			changed = true
			ax.peerCache[p.ID] = p
		}
	}

	if changed {
		log.Debugf("Peer listing has changed, recalculating configuration")
		ax.ParseWireguardConfig(ax.listenPort)
		ax.DeployWireguardConfig()
	}
	return nil
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
		if ifaceExists(darwinIface) {
			deleteDarwinIface()
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

// nodePrep add basic gathering and node condition checks here
func (ax *Apex) nodePrep() {

	// remove and existing wg interfaces
	if ax.os == Linux.String() && linkExists(wgIface) {
		if err := delLink(wgIface); err != nil {
			// not a fatal error since if this is on startup it could be absent
			log.Debugf("failed to delete netlink interface %s: %v", wgIface, err)
		}
	}
	if ax.os == Darwin.String() {
		if ifaceExists(darwinIface) {
			deleteDarwinIface()
		}
	}

	// discover the server reflexive address per ICE RFC8445 = (lol public address)
	stunAddr, err := GetPubIPv4()
	if err != nil {
		log.Infof("failed to query the stun server: %v", err)
	} else {
		var stunPresent bool
		if stunAddr != "" {
			stunPresent = true
		}
		if stunPresent {
			if err := ValidateIp(stunAddr); err == nil {
				ax.nodeReflexiveAddress = stunAddr
				log.Infof("the public facing ipv4 NAT address found for the host is: [ %s ]", stunAddr)
			}
		} else {
			log.Infof("no public facing NAT address found for the host")
		}
	}
}

func (ax *Apex) findLocalEndpointIp() (string, error) {
	// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
	// Otherwise, discover what the public of the node is and provide that to the peers unless the --internal flag was set,
	// in which case the endpoint address will be set to an existing address on the host.
	var localEndpointIP string
	// Darwin network discovery
	if !ax.stun && ax.os == Darwin.String() {
		controllerHost, controllerPort, err := net.SplitHostPort(ax.controllerURL.Host)
		if err != nil {
			log.Errorf("failed to split host:port endpoint pair: %v", err)
		}
		localEndpointIP, err = discoverGenericIPv4(controllerHost, controllerPort)
		if err != nil {
			return "", fmt.Errorf("%v", err)
		}
	}
	// Windows network discovery
	if !ax.stun && ax.os == Windows.String() {
		controllerHost, controllerPort, err := net.SplitHostPort(ax.controllerURL.Host)
		if err != nil {
			log.Errorf("failed to split host:port endpoint pair: %v", err)
		}
		localEndpointIP, err = discoverGenericIPv4(controllerHost, controllerPort)
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

// binaryChecks validate the required binaries are available
func binaryChecks() error {
	if GetOS() == Windows.String() {
		if !IsCommandAvailable(wgWinBinary) {
			return fmt.Errorf("%s command not found, is wireguard installed?", wgWinBinary)
		}
	} else {
		if !IsCommandAvailable(wgBinary) {
			return fmt.Errorf("%s command not found, is wireguard installed?", wgBinary)
		}
	}
	if GetOS() == Darwin.String() {
		if !IsCommandAvailable(wgGoBinary) {
			return fmt.Errorf("%s command not found, is wireguard installed?", wgGoBinary)
		}
	}
	return nil
}
