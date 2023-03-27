package nexodus

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"golang.org/x/term"
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
	WgWindowsConfPath = "C:/nexd/"
)

const (
	// when nexd is first starting up
	NexdStatusStarting = iota
	// when nexd is waiting for auth and the user must complete the OTP auth flow
	NexdStatusAuth
	// nexd is up and running normally
	NexdStatusRunning
)

var (
	invalidTokenGrant = errors.New("invalid_grant")
)

type Nexodus struct {
	wireguardPubKey         string
	wireguardPvtKey         string
	wireguardPubKeyInConfig bool
	tunnelIface             string
	controllerIP            string
	listenPort              int
	organization            uuid.UUID
	requestedIP             string
	userProvidedLocalIP     string
	LocalIP                 string
	childPrefix             []string
	stun                    bool
	relay                   bool
	discoveryNode           bool
	relayWgIP               string
	wgConfig                wgConfig
	client                  *client.Client
	controllerURL           *url.URL
	deviceCache             map[uuid.UUID]models.Device
	wgLocalAddress          string
	endpointLocalAddress    string
	nodeReflexiveAddress    string
	hostname                string
	symmetricNat            bool
	logger                  *zap.SugaredLogger
	// See the NexdStatus* constants
	status        int
	statusMsg     string
	version       string
	username      string
	password      string
	skipTlsVerify bool
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

func NewNexodus(ctx context.Context,
	logger *zap.SugaredLogger,
	controller string,
	username string,
	password string,
	wgListenPort int,
	wireguardPubKey string,
	wireguardPvtKey string,
	requestedIP string,
	userProvidedLocalIP string,
	childPrefix []string,
	stun bool,
	relay bool,
	discoveryNode bool,
	relayOnly bool,
	insecureSkipTlsVerify bool,
	version string,
) (*Nexodus, error) {
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

	ax := &Nexodus{
		wireguardPubKey:     wireguardPubKey,
		wireguardPvtKey:     wireguardPvtKey,
		controllerIP:        controller,
		listenPort:          wgListenPort,
		requestedIP:         requestedIP,
		userProvidedLocalIP: userProvidedLocalIP,
		childPrefix:         childPrefix,
		stun:                stun,
		relay:               relay,
		discoveryNode:       discoveryNode,
		deviceCache:         make(map[uuid.UUID]models.Device),
		controllerURL:       controllerURL,
		hostname:            hostname,
		symmetricNat:        relayOnly,
		logger:              logger,
		status:              NexdStatusStarting,
		version:             version,
		username:            username,
		password:            password,
		skipTlsVerify:       insecureSkipTlsVerify,
	}

	ax.tunnelIface = defaultTunnelDev()

	if ax.relay || ax.discoveryNode {
		ax.listenPort = WgDefaultPort
	}

	if err := ax.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	// remove orphaned wg interfaces from previous node joins
	ax.removeExistingInterface()

	if err := ax.symmetricNatDisco(); err != nil {
		ax.logger.Warn(err)
	}

	return ax, nil
}

func (ax *Nexodus) SetStatus(status int, msg string) {
	ax.statusMsg = msg
	ax.status = status
}

func (ax *Nexodus) Start(ctx context.Context, wg *sync.WaitGroup) error {
	var err error

	if err := ax.CtlServerStart(ctx, wg); err != nil {
		return fmt.Errorf("CtlServerStart(): %w", err)
	}

	var options []client.Option
	if ax.username == "" {
		options = append(options, client.WithDeviceFlow())
	} else if ax.username != "" && ax.password == "" {
		fmt.Print("Enter nexodus account password: ")
		passwdInput, err := term.ReadPassword(int(syscall.Stdin))
		println()
		if err != nil {
			return fmt.Errorf("login aborted: %w", err)
		}
		ax.password = string(passwdInput)
		options = append(options, client.WithPasswordGrant(ax.username, ax.password))
	} else {
		options = append(options, client.WithPasswordGrant(ax.username, ax.password))
	}
	if ax.skipTlsVerify { // #nosec G402
		options = append(options, client.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}

	ax.client, err = client.NewClient(ctx, ax.controllerURL.String(), func(msg string) {
		ax.SetStatus(NexdStatusAuth, msg)
	}, options...)
	if err != nil {
		return err
	}

	ax.SetStatus(NexdStatusRunning, "")

	if err := ax.handleKeys(); err != nil {
		return fmt.Errorf("handleKeys: %w", err)
	}

	user, err := ax.client.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("get organization error: %w", err)
	}

	if len(user.Organizations) == 0 {
		return fmt.Errorf("user does not belong to any organizations")
	}
	if len(user.Organizations) != 1 {
		return fmt.Errorf("user being in > 1 organization is not yet supported")
	}
	ax.logger.Infof("Device belongs in organization: %s", user.Organizations[0])
	ax.organization = user.Organizations[0]

	var localIP string
	var localEndpointPort int

	// User requested ip --request-ip takes precedent
	if ax.userProvidedLocalIP != "" {
		localIP = ax.userProvidedLocalIP
		localEndpointPort = ax.listenPort
	}

	if ax.relay || ax.discoveryNode {
		peerListing, err := ax.getPeerListing()
		if err != nil {
			return err
		}

		if ax.relay {
			existingRelay, err := ax.orgRelayCheck(peerListing)
			if err != nil {
				return err
			}
			if existingRelay != uuid.Nil {
				return fmt.Errorf("the organization already contains a relay node, device %s needs to be deleted before adding a new relay", existingRelay)
			}
		}

		if ax.discoveryNode {
			existingDiscoveryNode, err := ax.orgDiscoveryCheck(peerListing)
			if err != nil {
				return err
			}
			if existingDiscoveryNode != uuid.Nil {
				return fmt.Errorf("the organization already contains a discovery node, device %s needs to be deleted before adding a new discovery node", existingDiscoveryNode)
			}
		}
	}

	// If we are behind a symmetricNat, the endpoint ip discovered by a stun server is useless
	if !ax.symmetricNat && ax.stun && localIP == "" {
		ipPort, err := StunRequest(ax.logger, stunServer1, ax.listenPort)
		if err != nil {
			ax.logger.Warn("Unable to determine the public facing address, falling back to the local address")
		} else {
			localIP = ipPort.IP.String()
			localEndpointPort = ipPort.Port
		}
	}
	if localIP == "" {
		ip, err := ax.findLocalIP()
		if err != nil {
			return fmt.Errorf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %w", err)
		}
		localIP = ip
		localEndpointPort = ax.listenPort
	}
	ax.LocalIP = localIP
	ax.endpointLocalAddress = localIP
	endpointSocket := net.JoinHostPort(localIP, fmt.Sprintf("%d", localEndpointPort))
	device, err := ax.client.CreateDevice(models.AddDevice{
		UserID:                   user.ID,
		OrganizationID:           ax.organization,
		PublicKey:                ax.wireguardPubKey,
		LocalIP:                  endpointSocket,
		TunnelIP:                 ax.requestedIP,
		ChildPrefix:              ax.childPrefix,
		ReflexiveIPv4:            ax.nodeReflexiveAddress,
		EndpointLocalAddressIPv4: ax.endpointLocalAddress,
		SymmetricNat:             ax.symmetricNat,
		Hostname:                 ax.hostname,
		Relay:                    ax.relay,
	})
	if err != nil {
		var conflict client.ErrConflict
		if errors.As(err, &conflict) {
			deviceID, err := uuid.Parse(conflict.ID)
			if err != nil {
				return fmt.Errorf("error parsing conflicting device id: %w", err)
			}
			device, err = ax.client.UpdateDevice(deviceID, models.UpdateDevice{
				LocalIP:                  endpointSocket,
				ChildPrefix:              ax.childPrefix,
				ReflexiveIPv4:            ax.nodeReflexiveAddress,
				EndpointLocalAddressIPv4: ax.endpointLocalAddress,
				SymmetricNat:             &ax.symmetricNat,
				Hostname:                 ax.hostname,
			})
			if err != nil {
				return fmt.Errorf("error updating device: %w", err)
			}
		} else {
			return fmt.Errorf("error creating device: %w", err)
		}
	}
	ax.logger.Debug(fmt.Sprintf("Device: %+v", device))
	ax.logger.Infof("Successfully registered device with UUID: %+v", device.ID)

	// a hub router requires ip forwarding and iptables rules, OS type has already been checked
	if ax.relay {
		if err := enableForwardingIPv4(ax.logger); err != nil {
			return err
		}
		relayIpTables(ax.logger, ax.tunnelIface)
	}

	if err := ax.Reconcile(ax.organization, true); err != nil {
		return err
	}

	// send keepalives to all peers periodically
	if !ax.relay {
		util.GoWithWaitGroup(wg, func() {
			util.RunPeriodically(ctx, time.Second*10, func() {
				ax.Keepalive()
			})
		})
	}

	// gather wireguard state from the relay node periodically
	if ax.discoveryNode {
		util.GoWithWaitGroup(wg, func() {
			util.RunPeriodically(ctx, time.Second*30, func() {
				ax.logger.Debugf("Reconciling peers from relay state")
				if err := ax.discoveryStateReconcile(ax.organization); err != nil {
					ax.logger.Error(err)
				}
			})
		})
	}

	util.GoWithWaitGroup(wg, func() {
		util.RunPeriodically(ctx, pollInterval, func() {
			if err := ax.Reconcile(ax.organization, false); err != nil {
				// TODO: Add smarter reconciliation logic
				ax.logger.Errorf("Failed to reconcile state with the nexodus API server: %v", err)
				// if the token grant becomes invalid expires refresh or exit depending on the onboard method
				if strings.Contains(err.Error(), invalidTokenGrant.Error()) {
					if ax.username != "" {
						c, err := client.NewClient(ctx, ax.controllerURL.String(), func(msg string) {
							ax.SetStatus(NexdStatusAuth, msg)
						}, options...)
						if err != nil {
							ax.logger.Errorf("Failed to reconnect to the api-server, retrying in %v seconds: %v", pollInterval, err)
						} else {
							ax.client = c
							ax.SetStatus(NexdStatusRunning, "")
							ax.logger.Infoln("Nexodus agent has re-established a connection to the api-server")
						}
					} else {
						ax.logger.Fatalf("The token grant has expired due to an extended period offline, please " +
							"restart the agent for a one-time auth or login with --username --password to automatically reconnect")
					}
				}
			}
		})
	})

	return nil
}

func (ax *Nexodus) Keepalive() {
	ax.logger.Debug("Sending Keepalive")
	var peerEndpoints []string
	if !ax.relay {
		for _, value := range ax.deviceCache {
			nodeAddr := value.TunnelIP
			// strip the /32 from the prefix if present
			if net.ParseIP(value.TunnelIP) == nil {
				nodeIP, _, err := net.ParseCIDR(value.TunnelIP)
				nodeAddr = nodeIP.String()
				if err != nil {
					ax.logger.Debugf("failed parsing an ip from the prefix %v", err)
				}
			}
			peerEndpoints = append(peerEndpoints, nodeAddr)
		}
	}

	_ = probePeers(peerEndpoints, ax.logger)
}

func (ax *Nexodus) Reconcile(orgID uuid.UUID, firstTime bool) error {
	peerListing, err := ax.client.GetDeviceInOrganization(orgID)
	if err != nil {
		return err
	}
	var newPeers []models.Device
	if firstTime {
		// Initial peer list processing branches from here
		ax.logger.Debugf("Initializing peers for the first time")
		for _, p := range peerListing {
			existing, ok := ax.deviceCache[p.ID]
			if !ok {
				ax.deviceCache[p.ID] = p
				newPeers = append(newPeers, p)
			}
			if !reflect.DeepEqual(existing, p) {
				ax.deviceCache[p.ID] = p
				newPeers = append(newPeers, p)
			}
		}
		ax.buildPeersConfig()
		if err := ax.DeployWireguardConfig(newPeers, firstTime); err != nil {
			if errors.Is(err, interfaceErr) {
				ax.logger.Fatal(err)
			}
			return err
		}
	}
	// all subsequent peer listings updates get branched from here
	changed := false
	for _, p := range peerListing {
		existing, ok := ax.deviceCache[p.ID]
		if !ok {
			changed = true
			ax.deviceCache[p.ID] = p
			newPeers = append(newPeers, p)
		}
		if !reflect.DeepEqual(existing, p) {
			changed = true
			ax.deviceCache[p.ID] = p
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

// discoveryStateReconcile collect state from the discovery node and rejoin nodes with the dynamic state
func (ax *Nexodus) discoveryStateReconcile(orgID uuid.UUID) error {
	ax.logger.Debugf("Reconciling peers from relay state")
	peerListing, err := ax.client.GetDeviceInOrganization(orgID)
	if err != nil {
		return err
	}
	// get wireguard state from the discovery node to learn the dynamic reflexive ip:port socket
	discoInfo, err := DumpPeers(ax.tunnelIface)
	if err != nil {
		ax.logger.Errorf("eror dumping wg peers")
	}
	discoData := make(map[string]WgSessions)
	for _, peer := range discoInfo {
		_, ok := discoData[peer.PublicKey]
		if !ok {
			discoData[peer.PublicKey] = peer
		}
	}
	// re-join peers with updated state from the discovery node
	for _, peer := range peerListing {
		// if the peer is behind a symmetric NAT, skip to the next peer
		if peer.SymmetricNat {
			ax.logger.Debugf("skipping symmetric NAT node %s", peer.LocalIP)
			continue
		}
		_, ok := discoData[peer.PublicKey]
		if ok {
			if discoData[peer.PublicKey].Endpoint != "" {
				// test the reflexive address is valid and not still in a (none) state
				_, _, err := net.SplitHostPort(discoData[peer.PublicKey].Endpoint)
				if err != nil {
					// if the discovery state was not yet established or the peer is offline the endpoint can be (none)
					ax.logger.Debugf("failed to split host:port endpoint pair: %v", err)
					continue
				}
				endpointReflexiveAddress := discoData[peer.PublicKey].Endpoint
				// update the peer endpoint to the new reflexive address learned from the wg session
				_, err = ax.client.UpdateDevice(peer.ID, models.UpdateDevice{
					LocalIP: endpointReflexiveAddress,
				})
				if err != nil {
					ax.logger.Errorf("failed updating peer: %+v", err)
				}
			}
		}
	}
	return nil
}

// checkUnsupportedConfigs general matrix checks of required information or constraints to run the agent and join the mesh
func (ax *Nexodus) checkUnsupportedConfigs() error {
	if ax.relay && runtime.GOOS == Darwin.String() {
		return fmt.Errorf("OSX nodes cannot be a relay node, only Linux nodes")
	}
	if ax.relay && runtime.GOOS == Windows.String() {
		return fmt.Errorf("Windows nodes cannot be a relay node, only Linux nodes")
	}
	if ax.userProvidedLocalIP != "" {
		if err := ValidateIp(ax.userProvidedLocalIP); err != nil {
			return fmt.Errorf("the IP address passed in --local-endpoint-ip %s was not valid: %w", ax.userProvidedLocalIP, err)
		}
	}
	if ax.requestedIP != "" {
		if err := ValidateIp(ax.requestedIP); err != nil {
			return fmt.Errorf("the IP address passed in --request-ip %s was not valid: %w", ax.requestedIP, err)
		}
	}

	if ax.requestedIP != "" && ax.relay {
		ax.logger.Warnf("request-ip is currently unsupported for the relay node, a dynamic address will be used instead")
		ax.requestedIP = ""
	}

	for _, prefix := range ax.childPrefix {
		if err := ValidateCIDR(prefix); err != nil {
			return err
		}
	}
	return nil
}

// symmetricNatDisco determine if the joining node is within a symmetric NAT cone
func (ax *Nexodus) symmetricNatDisco() error {

	// discover the server reflexive address per ICE RFC8445
	stunAddr, err := StunRequest(ax.logger, stunServer1, ax.listenPort)
	if err != nil {
		return err
	} else {
		ax.nodeReflexiveAddress = stunAddr.IP.String()
	}

	isSymmetric := false
	stunAddr2, err := StunRequest(ax.logger, stunServer2, ax.listenPort)
	if err != nil {
		return err
	} else {
		isSymmetric = stunAddr.String() != stunAddr2.String()
	}

	if isSymmetric {
		ax.symmetricNat = true
		ax.logger.Infof("Symmetric NAT is detected, this node will be provisioned in relay mode only")
	}

	return nil
}

// orgRelayCheck checks if there is an existing Relay node in the organization that does not match this device's pub key
func (ax *Nexodus) orgRelayCheck(peerListing []models.Device) (uuid.UUID, error) {
	var relayID uuid.UUID
	for _, p := range peerListing {
		if p.Relay && ax.wireguardPubKey != p.PublicKey {
			return p.ID, nil
		}
	}

	return relayID, nil
}

// orgDiscoveryCheck checks if there is an existing Discovery node in the organization that does not match this device's pub key
func (ax *Nexodus) orgDiscoveryCheck(peerListing []models.Device) (uuid.UUID, error) {
	var discoveryID uuid.UUID
	for _, p := range peerListing {
		if p.Discovery && ax.wireguardPubKey != p.PublicKey {
			return p.ID, nil
		}
	}

	return discoveryID, nil
}

// getPeerListing return the peer listing for the current user account
func (ax *Nexodus) getPeerListing() ([]models.Device, error) {
	peerListing, err := ax.client.GetDeviceInOrganization(ax.organization)
	if err != nil {
		return nil, err
	}

	return peerListing, nil
}
