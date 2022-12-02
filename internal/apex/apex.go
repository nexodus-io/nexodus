package apex

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
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
	endpointIP              string
	childPrefix             string
	stun                    bool
	hubRouter               bool
	hubRouterWgIP           string
	os                      string
	wgConfig                wgConfig
	client                  *client.Client
	controllerURL           *url.URL
	// caches peers by their UUID
	peerCache map[uuid.UUID]models.Peer
	// maps device_ids to public keys
	keyCache             map[uuid.UUID]string
	wgLocalAddress       string
	endpointLocalAddress string
	nodeReflexiveAddress string
	hostname             string
	symmetricNat         bool
}

type wgConfig struct {
	Interface wgLocalConfig
	Peers     []wgPeerConfig `ini:"Peer,nonunique"`
}

type wgPeerConfig struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepAlive string
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

	// Force controller URL be api.${DOMAIN}
	controllerURL.Host = "api." + controllerURL.Host
	controllerURL.Path = ""

	withToken := cCtx.String("with-token")
	var option client.Option
	if withToken == "" {
		option = client.WithDeviceFlow()
	} else {
		option = client.WithToken(withToken)
	}

	client, err := client.NewClient(ctx, controllerURL.String(), option)
	if err != nil {
		log.Fatalf("error creating client: %+v", err)
	}

	if err := checkOS(); err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Error retrieving hostname: %+v", err)
	}

	var wgListenPort int
	if cCtx.Int("listen-port") == 0 {
		wgListenPort = getWgListenPort()
	}

	ax := &Apex{
		wireguardPubKey:        cCtx.String("public-key"),
		wireguardPvtKey:        cCtx.String("private-key"),
		controllerIP:           cCtx.String("controller"),
		controllerPasswd:       cCtx.String("controller-password"),
		listenPort:             wgListenPort,
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
		hostname:               hostname,
	}

	if ax.hubRouter {
		ax.listenPort = WgDefaultPort
	}

	if cCtx.Bool("relay-only") {
		log.Infof("Relay-only mode active")
		ax.symmetricNat = true
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

	device, err := ax.client.CreateDevice(ax.wireguardPubKey, ax.hostname)
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
	ax.endpointIP = localEndpointIP
	ax.endpointLocalAddress = localEndpointIP

	endpointSocket := fmt.Sprintf("%s:%d", localEndpointIP, ax.listenPort)

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
		ax.endpointLocalAddress,
		ax.symmetricNat,
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

	if err := ax.Reconcile(ax.zone, true); err != nil {
		log.Fatal(err)
	}

	// send keepalives to all peers on a ticker
	if !ax.hubRouter {
		go func() {
			keepAliveTicker := time.NewTicker(time.Second * 10)
			for range keepAliveTicker.C {
				ax.Keepalive()
			}
		}()
	}

	// gather wireguard state from the relay node on a ticker
	if ax.hubRouter {
		go func() {
			relayStateTicker := time.NewTicker(time.Second * 30)
			for range relayStateTicker.C {
				log.Debugf("Reconciling peers from relay state")
				ax.relayStateReconcile(user.ZoneID)
			}
		}()
	}

	ticker := time.NewTicker(pollInterval)
	for range ticker.C {
		if err := ax.Reconcile(user.ZoneID, false); err != nil {
			// TODO: Add smarter reconciliation logic
			// to handle disconnects and/or timeouts etc...
			log.Fatal(err)
		}
	}
}

func (ax *Apex) Keepalive() {
	log.Trace("Sending Keepalive")
	var peerEndpoints []string
	if !ax.hubRouter {
		for _, value := range ax.peerCache {
			peerEndpoints = append(peerEndpoints, value.NodeAddress)
		}
	}
	// basic discovery of what endpoints are reachable from the spoke peer that
	// determines whether to drain traffic to the hub or build a p2p peering
	// TODO: replace with a more in depth discovery than simple reachability
	_ = probePeers(peerEndpoints)
}

func (ax *Apex) Reconcile(zoneID uuid.UUID, firstTime bool) error {
	peerListing, err := ax.client.GetZonePeers(zoneID)
	if err != nil {
		return err
	}
	var newPeers []models.Peer
	if firstTime {
		// Initial peer list processing branches from here
		log.Debugf("Initializing peers for the first time")
		for _, p := range peerListing {
			existing, ok := ax.peerCache[p.ID]
			if !ok {
				ax.peerCache[p.ID] = p
				newPeers = append(newPeers, p)
			}
			if !reflect.DeepEqual(existing, p) {
				ax.peerCache[p.ID] = p
				newPeers = append(newPeers, p)
			}
		}
		ax.ParseWireguardConfig()
		ax.DeployWireguardConfig(newPeers, firstTime)
	}
	// all subsequent peer listings updates get branched from here
	changed := false
	for _, p := range peerListing {
		existing, ok := ax.peerCache[p.ID]
		if !ok {
			changed = true
			ax.peerCache[p.ID] = p
			newPeers = append(newPeers, p)
		}
		if !reflect.DeepEqual(existing, p) {
			changed = true
			ax.peerCache[p.ID] = p
			newPeers = append(newPeers, p)
		}
	}
	if changed {
		log.Debugf("Peers listing has changed, recalculating configuration")
		ax.ParseWireguardConfig()
		ax.DeployWireguardConfig(newPeers, false)
	}
	return nil
}

// relayStateReconcile collect state from the relay node and rejoin nodes with the dynamic state
func (ax *Apex) relayStateReconcile(zoneID uuid.UUID) {
	log.Debugf("Reconciling peers from relay state")
	peerListing, err := ax.client.GetZonePeers(zoneID)
	if err != nil {
		log.Errorf("Error getting a peer listing: %v", err)
	}
	// get wireguard state from the relay node to learn the dynamic reflexive ip:port socket
	relayInfo, err := DumpPeers(wgIface)
	if err != nil {
		log.Errorf("eror dumping wg peers")
	}
	relayData := make(map[string]WgSessions)
	for _, peerRelay := range relayInfo {
		_, ok := relayData[peerRelay.PublicKey]
		if !ok {
			relayData[peerRelay.PublicKey] = peerRelay
		}
	}
	// re-join peers with updated state from the relay node
	for _, peer := range peerListing {
		// if the peer is behind a symmetric NAT, skip to the next peer
		if peer.SymmetricNat {
			log.Debugf("skipping symmetric NAT node %s", peer.EndpointIP)
			continue
		}
		device, err := ax.client.GetDevice(peer.DeviceID)
		if err != nil {
			log.Fatalf("unable to get device %s: %s", peer.DeviceID, err)
		}
		_, ok := relayData[device.PublicKey]
		if ok {
			if relayData[device.PublicKey].Endpoint != "" {
				// set the reflexive address for the host based on the relay state
				endpointReflexiveAddress, _, err := net.SplitHostPort(relayData[device.PublicKey].Endpoint)
				if err != nil {
					// if the relay state was not yet established the endpoint can be (none)
					log.Infof("failed to split host:port endpoint pair: %v", err)
					continue
				}
				_, err = ax.client.CreatePeerInZone(zoneID, device.ID, relayData[device.PublicKey].Endpoint, ax.requestedIP,
					peer.ChildPrefix, false, false, "", endpointReflexiveAddress, peer.EnpointLocalAddressIPv4, peer.SymmetricNat)
				if err != nil {
					log.Errorf("failed creating peer: %+v", err)
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

	isSymmetric, err := IsSymmetricNAT()
	if err != nil {
		log.Error(err)
	}

	if isSymmetric {
		ax.symmetricNat = true
		log.Infof("Symmetric NAT is detected, this node will be provisioned in relay mode only")
	}

}

func (ax *Apex) findLocalEndpointIp() (string, error) {
	// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
	// Otherwise, discover what the public of the node is and provide that to the peers unless the --internal flag was set,
	// in which case the endpoint address will be set to an existing address on the host.
	var localEndpointIP string
	var err error
	// Darwin network discovery
	if !ax.stun && ax.os == Darwin.String() {
		localEndpointIP, err = discoverGenericIPv4(ax.controllerURL.Host, "80")
		if err != nil {
			return "", fmt.Errorf("%v", err)
		}
	}
	// Windows network discovery
	if !ax.stun && ax.os == Windows.String() {
		localEndpointIP, err = discoverGenericIPv4(ax.controllerURL.Host, "80")
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
