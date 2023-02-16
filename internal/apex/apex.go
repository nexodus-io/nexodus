package apex

import (
	"context"
	"errors"
	"fmt"
	"github.com/redhat-et/apex/internal/util"
	"net"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/client"
	"github.com/redhat-et/apex/internal/models"
	log "github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

const (
	pollInterval      = 5 * time.Second
	wgBinary          = "wg"
	wgGoBinary        = "wireguard-go"
	wgWinBinary       = "wireguard.exe"
	WgLinuxConfPath   = "/etc/wireguard/"
	WgDarwinConfPath  = "/usr/local/etc/wireguard/"
	darwinIface       = "utun8"
	WgDefaultPort     = 51820
	wgIface           = "wg0"
	WgWindowsConfPath = "C:/apex/"
)

type Apex struct {
	wireguardPubKey         string
	wireguardPvtKey         string
	wireguardPubKeyInConfig bool
	tunnelIface             string
	controllerIP            string
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
	logger               *zap.SugaredLogger
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

func NewApex(ctx context.Context, logger *zap.SugaredLogger,
	controller string,
	username string,
	password string,
	wgListenPort int,
	wireguardPubKey string,
	wireguardPvtKey string,
	requestedIP string,
	userProvidedEndpointIP string,
	childPrefix string,
	stun bool,
	hubRouter bool,
	relayOnly bool,
) (*Apex, error) {
	if err := binaryChecks(); err != nil {
		return nil, err
	}

	controllerURL, err := url.Parse(controller)
	if err != nil {
		return nil, err
	}

	// Force controller URL be api.${DOMAIN}
	controllerURL.Host = "api." + controllerURL.Host
	controllerURL.Path = ""

	var option client.Option
	if username == "" {
		option = client.WithDeviceFlow()
	} else {
		option = client.WithPasswordGrant(username, password)
	}

	client, err := client.NewClient(ctx, controllerURL.String(), option)
	if err != nil {
		return nil, err
	}

	if err := checkOS(logger); err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	if wgListenPort == 0 {
		wgListenPort, err = getWgListenPort()
		if err != nil {
			return nil, err
		}
	}

	ax := &Apex{
		wireguardPubKey:        wireguardPubKey,
		wireguardPvtKey:        wireguardPvtKey,
		controllerIP:           controller,
		listenPort:             wgListenPort,
		requestedIP:            requestedIP,
		userProvidedEndpointIP: userProvidedEndpointIP,
		childPrefix:            childPrefix,
		stun:                   stun,
		hubRouter:              hubRouter,
		client:                 client,
		os:                     GetOS(),
		peerCache:              make(map[uuid.UUID]models.Peer),
		keyCache:               make(map[uuid.UUID]string),
		controllerURL:          controllerURL,
		hostname:               hostname,
		symmetricNat:           relayOnly,
		logger:                 logger,
	}

	ax.tunnelIface = defaultTunnelDev(ax.os)

	if ax.hubRouter {
		ax.listenPort = WgDefaultPort
	}

	if err := ax.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	ax.nodePrep()

	return ax, nil
}

func (ax *Apex) Start(ctx context.Context, wg *sync.WaitGroup) error {
	var err error

	if err := ax.handleKeys(); err != nil {
		return fmt.Errorf("handleKeys: %w", err)
	}

	device, err := ax.client.CreateDevice(ax.wireguardPubKey, ax.hostname)
	if err != nil {
		return fmt.Errorf("device register error: %w", err)
	}
	ax.logger.Infof("Device Registered with UUID: %s", device.ID)

	user, err := ax.client.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("get zone error: %w", err)
	}
	ax.logger.Infof("Device belongs in zone: %s", user.ZoneID)
	ax.zone = user.ZoneID

	var localEndpointIP string
	var localEndpointPort int

	// User requested ip --request-ip takes precedent
	if ax.userProvidedEndpointIP != "" {
		localEndpointIP = ax.userProvidedEndpointIP
		localEndpointPort = ax.listenPort
	}

	// If we are behind a symmetricNat, the endpoint ip discovered by a stun server is useless
	if !ax.symmetricNat && ax.stun && localEndpointIP == "" {
		ipPort, err := StunRequest(ax.logger, stunServer, ax.listenPort)
		if err != nil {
			ax.logger.Warn("Unable to determine the public facing address, falling back to the local address")
		} else {
			localEndpointIP = ipPort.IP.String()
			localEndpointPort = ipPort.Port
		}
	}
	if localEndpointIP == "" {
		ip, err := ax.findLocalEndpointIp()
		if err != nil {
			return fmt.Errorf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %w", err)
		}
		localEndpointIP = ip
		localEndpointPort = ax.listenPort
	}
	ax.endpointIP = localEndpointIP
	ax.endpointLocalAddress = localEndpointIP

	endpointSocket := fmt.Sprintf("%s:%d", localEndpointIP, localEndpointPort)

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
		return fmt.Errorf("error creating peer: %w", err)
	}
	ax.logger.Info("Successfully registered with Apex Controller")

	// a hub router requires ip forwarding and iptables rules, OS type has already been checked
	if ax.hubRouter {
		if err := enableForwardingIPv4(ax.logger); err != nil {
			return err
		}
		hubRouterIpTables(ax.logger, ax.tunnelIface)
	}

	if err := ax.Reconcile(ax.zone, true); err != nil {
		return err
	}

	// send keepalives to all peers periodically
	if !ax.hubRouter {
		util.GoWithWaitGroup(wg, func() {
			util.RunPeriodically(ctx, time.Second*10, func() {
				ax.Keepalive()
			})
		})
	}

	// gather wireguard state from the relay node periodically
	if ax.hubRouter {
		util.GoWithWaitGroup(wg, func() {
			util.RunPeriodically(ctx, time.Second*30, func() {
				ax.logger.Debugf("Reconciling peers from relay state")
				if err := ax.relayStateReconcile(user.ZoneID); err != nil {
					ax.logger.Error(err)
				}
			})
		})
	}

	util.GoWithWaitGroup(wg, func() {
		util.RunPeriodically(ctx, pollInterval, func() {
			if err := ax.Reconcile(user.ZoneID, false); err != nil {
				// TODO: Add smarter reconciliation logic
				// to handle disconnects and/or timeouts etc...
				ax.logger.Errorf("Failed to reconcile state with the apex API server: ", err)
			}
		})
	})

	return nil
}

func (ax *Apex) Keepalive() {
	ax.logger.Debug("Sending Keepalive")
	var peerEndpoints []string
	if !ax.hubRouter {
		for _, value := range ax.peerCache {
			nodeAddr := value.NodeAddress
			// strip the /32 from the prefix if present
			if net.ParseIP(value.NodeAddress) == nil {
				nodeIP, _, err := net.ParseCIDR(value.NodeAddress)
				nodeAddr = nodeIP.String()
				if err != nil {
					ax.logger.Debugf("failed parsing an ip from the prefix %v", err)
				}
			}
			peerEndpoints = append(peerEndpoints, nodeAddr)
		}
	}

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
		ax.logger.Debugf("Initializing peers for the first time")
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
		ax.buildPeersConfig()
		if err := ax.DeployWireguardConfig(newPeers, firstTime); err != nil {
			if errors.Is(err, interfaceErr) {
				log.Fatal(err)
			}
			return err
		}
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
		ax.logger.Debugf("Peers listing has changed, recalculating configuration")
		ax.buildPeersConfig()
		if err := ax.DeployWireguardConfig(newPeers, false); err != nil {
			return err
		}
	}

	// check for any peer deletions
	if err := ax.handlePeerDelete(peerListing); err != nil {
		ax.logger.Error(err)
	}

	return nil
}

// relayStateReconcile collect state from the relay node and rejoin nodes with the dynamic state
func (ax *Apex) relayStateReconcile(zoneID uuid.UUID) error {
	ax.logger.Debugf("Reconciling peers from relay state")
	peerListing, err := ax.client.GetZonePeers(zoneID)
	if err != nil {
		log.Errorf("Error getting a peer listing: %v", err)
	}
	// get wireguard state from the relay node to learn the dynamic reflexive ip:port socket
	relayInfo, err := DumpPeers(ax.tunnelIface)
	if err != nil {
		log.Errorf("error dumping wg peers")
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
			ax.logger.Debugf("skipping symmetric NAT node %s", peer.EndpointIP)
			continue
		}
		device, err := ax.client.GetDevice(peer.DeviceID)
		if err != nil {
			return fmt.Errorf("unable to get device %s: %w", peer.DeviceID, err)
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
					peer.ChildPrefix, false, false, "", endpointReflexiveAddress, peer.EndpointLocalAddressIPv4, peer.SymmetricNat)
				if err != nil {
					log.Errorf("failed creating peer: %v", err)
				}
			}
		}
	}
	return nil
}

// Check OS and report error if the OS is not supported.
func checkOS(logger *zap.SugaredLogger) error {
	nodeOS := GetOS()
	switch nodeOS {
	case Darwin.String():
		logger.Debugf("[%s] operating system detected", nodeOS)
		// ensure the osx wireguard directory exists
		if err := CreateDirectory(WgDarwinConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %w", WgDarwinConfPath, err)
		}
	case Windows.String():
		logger.Debugf("[%s] operating system detected", nodeOS)
		// ensure the windows wireguard directory exists
		if err := CreateDirectory(WgWindowsConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %w", WgWindowsConfPath, err)
		}
	case Linux.String():
		logger.Debugf("[%s] operating system detected", nodeOS)
		// ensure the linux wireguard directory exists
		if err := CreateDirectory(WgLinuxConfPath); err != nil {
			return fmt.Errorf("unable to create the wireguard config directory [%s]: %w", WgLinuxConfPath, err)
		}
	default:
		return fmt.Errorf("OS [%s] is not supported\n", runtime.GOOS)
	}
	return nil
}

// checkUnsupportedConfigs general matrix checks of required information or constraints to run the agent and join the mesh
func (ax *Apex) checkUnsupportedConfigs() error {
	if ax.hubRouter && ax.os == Darwin.String() {
		return fmt.Errorf("OSX nodes cannot be a hub-router, only Linux nodes")
	}
	if ax.hubRouter && ax.os == Windows.String() {
		return fmt.Errorf("Windows nodes cannot be a hub-router, only Linux nodes")
	}
	if ax.userProvidedEndpointIP != "" {
		if err := ValidateIp(ax.userProvidedEndpointIP); err != nil {
			return fmt.Errorf("the IP address passed in --local-endpoint-ip %s was not valid: %w", ax.userProvidedEndpointIP, err)
		}
	}
	if ax.requestedIP != "" {
		if err := ValidateIp(ax.requestedIP); err != nil {
			return fmt.Errorf("the IP address passed in --request-ip %s was not valid: %w", ax.requestedIP, err)
		}
	}
	if ax.childPrefix != "" {
		if err := ValidateCIDR(ax.childPrefix); err != nil {
			return fmt.Errorf("the CIDR prefix passed in --child-prefix %s was not valid: %w", ax.childPrefix, err)
		}
	}
	return nil
}

// nodePrep add basic gathering and node condition checks here
func (ax *Apex) nodePrep() {

	// remove an existing wg interfaces
	if ax.os == Linux.String() && linkExists(ax.tunnelIface) {
		if err := delLink(ax.tunnelIface); err != nil {
			// not a fatal error since if this is on startup it could be absent
			ax.logger.Debugf("failed to delete netlink interface %s: %v", ax.tunnelIface, err)
		}
	}
	if ax.os == Darwin.String() {
		if ifaceExists(ax.logger, ax.tunnelIface) {
			deleteDarwinIface(ax.logger, ax.tunnelIface)
		}
	}

	// discover the server reflexive address per ICE RFC8445 = (lol public address)
	stunAddr, err := StunRequest(ax.logger, stunServer1, ax.listenPort)
	if err != nil {
		log.Infof("failed to query the stun server: %v", err)
	} else {
		ax.nodeReflexiveAddress = stunAddr.IP.String()
	}

	isSymmetric := false
	stunAddr2, err := StunRequest(ax.logger, stunServer2, ax.listenPort)
	if err != nil {
		log.Error(err)
	} else {
		isSymmetric = stunAddr.String() != stunAddr2.String()
	}

	if isSymmetric {
		ax.symmetricNat = true
		log.Infof("Symmetric NAT is detected, this node will be provisioned in relay mode only")
	}

}

func (ax *Apex) findLocalEndpointIp() (string, error) {
	// If the user supplied what they want the local endpoint IP to be, use that (enables privateIP <--> privateIP peering).
	// Otherwise, discover what the public ip of the node is and provide that to the peers.
	var localEndpointIP string
	var err error
	// Darwin/Windows network discovery
	if ax.os == Darwin.String() || ax.os == Windows.String() {
		localEndpointIP, err = discoverGenericIPv4(ax.logger, ax.controllerURL.Host, "443")
		if err != nil {
			return "", fmt.Errorf("%w", err)
		}
	}

	// Linux network discovery
	if ax.os == Linux.String() {
		linuxIP, err := discoverLinuxAddress(ax.logger, 4)
		if err != nil {
			return "", fmt.Errorf("%w", err)
		}
		localEndpointIP = linuxIP.String()
	}
	return localEndpointIP, nil
}

// binaryChecks validate the required binaries are available
func binaryChecks() error {
	// Windows userspace binary
	if GetOS() == Windows.String() {
		if !IsCommandAvailable(wgWinBinary) {
			return fmt.Errorf("%s command not found, is wireguard installed?", wgWinBinary)
		}
	}
	// Darwin wireguard-go userspace binary
	if GetOS() == Darwin.String() {
		if !IsCommandAvailable(wgGoBinary) {
			return fmt.Errorf("%s command not found, is wireguard installed?", wgGoBinary)
		}
	}
	// all OSs require the wg binary
	if !IsCommandAvailable(wgBinary) {
		return fmt.Errorf("%s command not found, is wireguard installed?", wgBinary)
	}
	return nil
}
