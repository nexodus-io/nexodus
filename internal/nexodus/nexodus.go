package nexodus

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"golang.org/x/term"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

const (
	pollInterval       = 5 * time.Second
	wgBinary           = "wg"
	wgGoBinary         = "wireguard-go"
	wgWinBinary        = "wireguard.exe"
	WgLinuxConfPath    = "/etc/wireguard/"
	WgDarwinConfPath   = "/usr/local/etc/wireguard/"
	darwinIface        = "utun8"
	WgDefaultPort      = 51820
	wgIface            = "wg0"
	WgWindowsConfPath  = "C:/nexd/"
	wgOrgIPv6PrefixLen = "64"
)

const (
	// when nexd is first starting up
	NexdStatusStarting = iota
	// when nexd is waiting for auth and the user must complete the OTP auth flow
	NexdStatusAuth
	// nexd is up and running normally
	NexdStatusRunning
)

const (
	stunServer1 = "stun1.l.google.com:19302"
	stunServer2 = "stun2.l.google.com:19302"
)

var (
	invalidTokenGrant = errors.New("invalid_grant")
)

// embedded in Nexodus struct
type userspaceWG struct {
	userspaceMode bool
	userspaceTun  tun.Device
	userspaceNet  *netstack.Net
	userspaceDev  *device.Device
	// the last address configured on the userspace wireguard interface
	userspaceLastAddress string
	ingresProxies        []*UsProxy
	egressProxies        []*UsProxy
}

type Nexodus struct {
	wireguardPubKey          string
	wireguardPvtKey          string
	wireguardPubKeyInConfig  bool
	tunnelIface              string
	controllerIP             string
	listenPort               int
	organization             string
	requestedIP              string
	userProvidedLocalIP      string
	TunnelIP                 string
	TunnelIpV6               string
	childPrefix              []string
	stun                     bool
	relay                    bool
	discoveryNode            bool
	relayWgIP                string
	wgConfig                 wgConfig
	client                   *client.APIClient
	controllerURL            *url.URL
	deviceCache              map[string]public.ModelsDevice
	endpointLocalAddress     string
	nodeReflexiveAddressIPv4 netip.AddrPort
	hostname                 string
	symmetricNat             bool
	ipv6Supported            bool
	os                       string
	logger                   *zap.SugaredLogger
	// See the NexdStatus* constants
	status        int
	statusMsg     string
	version       string
	username      string
	password      string
	skipTlsVerify bool
	stateDir      string
	userspaceWG
}

type wgConfig struct {
	Interface wgLocalConfig
	Peers     []wgPeerConfig
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

func NewNexodus(
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
	userspaceMode bool,
	stateDir string,
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
		deviceCache:         make(map[string]public.ModelsDevice),
		controllerURL:       controllerURL,
		hostname:            hostname,
		symmetricNat:        relayOnly,
		logger:              logger,
		status:              NexdStatusStarting,
		version:             version,
		username:            username,
		password:            password,
		skipTlsVerify:       insecureSkipTlsVerify,
		stateDir:            stateDir,
	}
	ax.userspaceMode = userspaceMode
	ax.tunnelIface = ax.defaultTunnelDev()

	if ax.relay || ax.discoveryNode {
		ax.listenPort = WgDefaultPort
	}

	if err := ax.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	if err := prepOS(logger); err != nil {
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

	// TODO
	//
	// We must deal with a lack of permissions and the possibility for there
	// to be more than one instance of nexd running at the same time in this mode
	// before we can enable it in this mode.
	if !ax.userspaceMode {
		if err := ax.CtlServerStart(ctx, wg); err != nil {
			return fmt.Errorf("CtlServerStart(): %w", err)
		}
	}
	var options []client.Option
	if ax.stateDir != "" {
		options = append(options, client.WithTokenFile(filepath.Join(ax.stateDir, "apitoken.json")))
	}
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

	ax.client, err = client.NewAPIClient(ctx, ax.controllerURL.String(), func(msg string) {
		ax.SetStatus(NexdStatusAuth, msg)
	}, options...)
	if err != nil {
		return err
	}

	ax.SetStatus(NexdStatusRunning, "")

	if err := ax.handleKeys(); err != nil {
		return fmt.Errorf("handleKeys: %w", err)
	}

	user, _, err := ax.client.UsersApi.GetUser(ctx, "me").Execute()
	if err != nil {
		return fmt.Errorf("get user error: %w", err)
	}

	organizations, _, err := ax.client.OrganizationsApi.ListOrganizations(ctx).Execute()
	if err != nil {
		return fmt.Errorf("get organizations error: %w", err)
	}

	if len(organizations) == 0 {
		return fmt.Errorf("user does not belong to any organizations")
	}
	if len(organizations) != 1 {
		return fmt.Errorf("user being in > 1 organization is not yet supported")
	}
	ax.logger.Infof("Device belongs in organization: %s (%s)", organizations[0].Name, organizations[0].Id)
	ax.organization = organizations[0].Id

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
			if existingRelay != "" {
				return fmt.Errorf("the organization already contains a relay node, device %s needs to be deleted before adding a new relay", existingRelay)
			}
		}

		if ax.discoveryNode {
			existingDiscoveryNode, err := ax.orgDiscoveryCheck(peerListing)
			if err != nil {
				return err
			}
			if existingDiscoveryNode != "" {
				return fmt.Errorf("the organization already contains a discovery node, device %s needs to be deleted before adding a new discovery node", existingDiscoveryNode)
			}
		}
	}

	// If we are behind a symmetricNat, the endpoint ip discovered by a stun server is useless
	if !ax.symmetricNat && ax.stun && localIP == "" {
		ipPort, err := stunRequest(ax.logger, stunServer1, ax.listenPort)
		if err != nil {
			ax.logger.Warn("Unable to determine the public facing address, falling back to the local address")
		} else {
			localIP = ipPort.Addr().String()
			localEndpointPort = int(ipPort.Port())
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

	ax.os = runtime.GOOS

	ax.endpointLocalAddress = localIP
	endpointSocket := net.JoinHostPort(localIP, fmt.Sprintf("%d", localEndpointPort))
	device, _, err := ax.client.DevicesApi.CreateDevice(context.Background()).Device(public.ModelsAddDevice{
		UserId:                  user.Id,
		OrganizationId:          ax.organization,
		PublicKey:               ax.wireguardPubKey,
		LocalIp:                 endpointSocket,
		TunnelIp:                ax.requestedIP,
		ChildPrefix:             ax.childPrefix,
		ReflexiveIp4:            ax.nodeReflexiveAddressIPv4.String(),
		EndpointLocalAddressIp4: ax.endpointLocalAddress,
		SymmetricNat:            ax.symmetricNat,
		Hostname:                ax.hostname,
		Relay:                   ax.relay,
		Os:                      ax.os,
	}).Execute()
	if err != nil {
		var apiError *public.GenericOpenAPIError
		if errors.As(err, &apiError) {
			switch model := apiError.Model().(type) {
			case public.ModelsConflictsError:
				device, _, err = ax.client.DevicesApi.UpdateDevice(context.Background(), model.Id).Update(public.ModelsUpdateDevice{
					LocalIp:                 endpointSocket,
					ChildPrefix:             ax.childPrefix,
					ReflexiveIp4:            ax.nodeReflexiveAddressIPv4.String(),
					EndpointLocalAddressIp4: ax.endpointLocalAddress,
					SymmetricNat:            ax.symmetricNat,
					Hostname:                ax.hostname,
				}).Execute()
				if err != nil {
					return fmt.Errorf("error updating device: %w", err)
				}
			default:
				return fmt.Errorf("error creating device: %w", err)
			}
		} else {
			return fmt.Errorf("error creating device: %w", err)
		}
	}
	ax.logger.Debug(fmt.Sprintf("Device: %+v", device))
	ax.logger.Infof("Successfully registered device with UUID: %+v", device.Id)

	// a relay node requires ip forwarding and nftable rules, OS type has already been checked
	if ax.relay {
		if err := ax.relayPrep(); err != nil {
			return err
		}
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
						c, err := client.NewAPIClient(ctx, ax.controllerURL.String(), func(msg string) {
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

	// Experimental STUN discovery
	if runtime.GOOS == Linux.String() {
		util.GoWithWaitGroup(wg, func() {
			util.RunPeriodically(ctx, time.Second*20, func() {
				if err := ax.reconcileStun(device.Id); err != nil {
					ax.logger.Debug(err)
				}
			})
		})
	}

	for _, proxy := range ax.ingresProxies {
		proxy.Start(ctx, wg, ax.userspaceNet)
	}
	for _, proxy := range ax.egressProxies {
		proxy.Start(ctx, wg, ax.userspaceNet)
	}

	return nil
}

func (ax *Nexodus) Keepalive() {
	if ax.userspaceMode {
		ax.logger.Debugf("Keepalive not yet implemented in userspace mode")
		return
	}
	ax.logger.Debug("Sending Keepalive")
	var peerEndpoints []string
	if !ax.relay {
		for _, value := range ax.deviceCache {
			nodeAddr := value.TunnelIp
			// strip the /32 from the prefix if present
			if net.ParseIP(value.TunnelIp) == nil {
				nodeIP, _, err := net.ParseCIDR(value.TunnelIp)
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

func (ax *Nexodus) reconcileStun(deviceID string) error {
	reflexiveIP, err := stunRequest(ax.logger, stunServer1, ax.listenPort)
	if err != nil {
		return fmt.Errorf("bpf stun request error: %w", err)
	}

	if ax.nodeReflexiveAddressIPv4 != reflexiveIP {
		ax.logger.Infof("detected a NAT binding changed for this device %s from %s to %s, updating peers", deviceID, ax.nodeReflexiveAddressIPv4, reflexiveIP)
		res, _, err := ax.client.DevicesApi.UpdateDevice(context.Background(), deviceID).Update(public.ModelsUpdateDevice{
			LocalIp:      reflexiveIP.String(),
			ReflexiveIp4: reflexiveIP.String(),
		}).Execute()
		if err != nil {
			return fmt.Errorf("failed to update this device's new NAT binding, likely still reconnecting to the api-server, retrying in 20s: %w", err)
		} else {
			ax.logger.Debugf("update device response %+v", res)
			ax.nodeReflexiveAddressIPv4 = reflexiveIP
		}
	}
	ax.logger.Debugf("relfexive binding is %s", reflexiveIP)

	return nil
}

func (ax *Nexodus) Reconcile(orgID string, firstTime bool) error {
	peerListing, _, err := ax.client.DevicesApi.ListDevicesInOrganization(context.Background(), orgID).Execute()
	if err != nil {
		return err
	}
	var newPeers []public.ModelsDevice
	if firstTime {
		// Initial peer list processing branches from here
		ax.logger.Debugf("Initializing peers for the first time")
		for _, p := range peerListing {
			existing, ok := ax.deviceCache[p.Id]
			if !ok {
				ax.deviceCache[p.Id] = p
				newPeers = append(newPeers, p)
			}
			if !reflect.DeepEqual(existing, p) {
				ax.deviceCache[p.Id] = p
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
		existing, ok := ax.deviceCache[p.Id]
		if !ok {
			changed = true
			ax.deviceCache[p.Id] = p
			newPeers = append(newPeers, p)
		}
		if !reflect.DeepEqual(existing, p) {
			changed = true
			ax.deviceCache[p.Id] = p
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
func (ax *Nexodus) discoveryStateReconcile(orgID string) error {
	ax.logger.Debugf("Reconciling peers from relay state")
	peerListing, _, err := ax.client.DevicesApi.ListDevicesInOrganization(context.Background(), orgID).Execute()
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
		// Uncomment to update Linux nodes that are getting reflexive changes via STUN as well
		//if peer.Os == Linux.String() {
		//	ax.logger.Debug("skipping Linux node")
		//	continue
		//}
		// if the peer is behind a symmetric NAT, skip to the next peer
		if peer.SymmetricNat {
			ax.logger.Debugf("skipping symmetric NAT node %s", peer.LocalIp)
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
				_, _, err = ax.client.DevicesApi.UpdateDevice(context.Background(), peer.Id).Update(public.ModelsUpdateDevice{
					LocalIp: endpointReflexiveAddress,
				}).Execute()

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
		if util.IsIPv6Address(ax.requestedIP) {
			return fmt.Errorf("--request-ip only supports IPv4 addresses")
		}
	}

	if ax.requestedIP != "" && ax.relay {
		ax.logger.Warnf("request-ip is currently unsupported for the relay node, a dynamic address will be used instead")
		ax.requestedIP = ""
	}

	if len(ax.childPrefix) > 0 && ax.relay {
		return fmt.Errorf("child-prefixes are not supported on relay nodes")
	}

	for _, prefix := range ax.childPrefix {
		if err := ValidateCIDR(prefix); err != nil {
			return err
		}
		// TODO: enable child-prefix-v6
		if util.IsIPv6Prefix(prefix) {
			return fmt.Errorf("currently --child-prefix only supports IPv4 prefixes")
		}
	}

	if !isIPv6Supported() {
		ax.ipv6Supported = false
		ax.logger.Warn("IPv6 does not appear to be enabled on this host, only IPv4 will be provisioned or restart nexd with IPv6 enabled on this host")
	} else {
		ax.ipv6Supported = true
	}

	return nil
}

// symmetricNatDisco determine if the joining node is within a symmetric NAT cone
func (ax *Nexodus) symmetricNatDisco() error {

	// discover the server reflexive address per ICE RFC8445
	stunAddr, err := stunRequest(ax.logger, stunServer1, ax.listenPort)
	if err != nil {
		return err
	} else {
		ax.nodeReflexiveAddressIPv4 = stunAddr
	}

	isSymmetric := false
	stunAddr2, err := stunRequest(ax.logger, stunServer2, ax.listenPort)
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
func (ax *Nexodus) orgRelayCheck(peerListing []public.ModelsDevice) (string, error) {
	for _, p := range peerListing {
		if p.Relay && ax.wireguardPubKey != p.PublicKey {
			return p.Id, nil
		}
	}
	return "", nil
}

// orgDiscoveryCheck checks if there is an existing Discovery node in the organization that does not match this device's pub key
func (ax *Nexodus) orgDiscoveryCheck(peerListing []public.ModelsDevice) (string, error) {
	for _, p := range peerListing {
		if p.Discovery && ax.wireguardPubKey != p.PublicKey {
			return p.Id, nil
		}
	}
	return "", nil
}

// getPeerListing return the peer listing for the current user account
func (ax *Nexodus) getPeerListing() ([]public.ModelsDevice, error) {
	peerListing, _, err := ax.client.DevicesApi.ListDevicesInOrganization(context.Background(), ax.organization).Execute()
	if err != nil {
		return nil, err
	}
	return peerListing, nil
}

func (ax *Nexodus) setupInterface() error {
	if ax.userspaceMode {
		return ax.setupInterfaceUS()
	}
	return ax.setupInterfaceOS()
}

func (ax *Nexodus) defaultTunnelDev() string {
	if ax.userspaceMode {
		return ax.defaultTunnelDevUS()
	}
	return defaultTunnelDevOS()
}
