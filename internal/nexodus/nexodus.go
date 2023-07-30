package nexodus

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/state"
	"golang.org/x/oauth2"
	"net"
	"net/http"
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

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/stun"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"golang.org/x/term"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

const (
	pollInterval       = 5 * time.Second
	wgGoBinary         = "wireguard-go"
	nexdWgGoBinary     = "nexd-wireguard-go"
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
	// retry interval for api server retries
	retryInterval = 15 * time.Second
	// max retries for api server retries
	maxRetries = 3
)

var (
	invalidTokenGrant = errors.New("invalid_grant")
	invalidToken      = errors.New("invalid_token")
)

// embedded in Nexodus struct
type userspaceWG struct {
	userspaceMode bool
	userspaceTun  tun.Device
	userspaceNet  *netstack.Net
	userspaceDev  *device.Device
	// the last address configured on the userspace wireguard interface
	userspaceLastAddress string
	proxyLock            sync.RWMutex
	proxies              map[ProxyKey]*UsProxy
}

// Threasholds for determining peer connection health
const ()

type peerHealth struct {
	// the time we started tracking this data
	startTime time.Time
	// the last tx bytes value for this peer
	lastTxBytes int64
	// the time of the last tx bytes update
	lastTxTime time.Time
	// the last rx bytes value for this peer
	lastRxBytes int64
	// the time of the last rx bytes update
	lastRxTime time.Time
	// the last time a handshake occurred with this peer
	lastHandshakeTime time.Time
	// last handshake time in raw form
	lastHandshake string
	// last time this data was refreshed
	lastRefresh time.Time
	// the configured endpoint for this peer
	endpoint string
	// whether we see this peer connection as healthy, see peerIsHealthy()
	peerHealthy bool
}

type deviceCacheEntry struct {
	device public.ModelsDevice
	// the last time this device was updated
	lastUpdated time.Time
	peerHealth
}

type Nexodus struct {
	wireguardPubKey          string
	wireguardPvtKey          string
	wireguardPubKeyInConfig  bool
	tunnelIface              string
	listenPort               int
	orgId                    string
	org                      *public.ModelsOrganization
	requestedIP              string
	userProvidedLocalIP      string
	TunnelIP                 string
	TunnelIpV6               string
	childPrefix              []string
	stun                     bool
	relay                    bool
	networkRouter            bool
	networkRouterDisableNAT  bool
	netRouterInterfaceMap    map[string]*net.Interface
	relayWgIP                string
	wgConfig                 wgConfig
	client                   *client.APIClient
	apiURL                   *url.URL
	deviceCacheLock          sync.RWMutex
	deviceCache              map[string]deviceCacheEntry
	endpointLocalAddress     string
	nodeReflexiveAddressIPv4 netip.AddrPort
	hostname                 string
	securityGroup            *public.ModelsSecurityGroup
	symmetricNat             bool
	ipv6Supported            bool
	os                       string
	logger                   *zap.SugaredLogger
	logLevel                 *zap.AtomicLevel
	// See the NexdStatus* constants
	status        int
	statusMsg     string
	version       string
	username      string
	password      string
	skipTlsVerify bool
	stateStore    state.Store
	userspaceWG
	informer     *public.ApiListDevicesInOrganizationInformer
	informerStop context.CancelFunc
	nexCtx       context.Context
	nexWg        *sync.WaitGroup
}

type wgConfig struct {
	Interface wgLocalConfig
	Peers     map[string]wgPeerConfig
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
	logLevel *zap.AtomicLevel,
	apiURL *url.URL,
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
	relayOnly bool,
	networkRouterNode bool,
	networkRouterDisableNAT bool,
	insecureSkipTlsVerify bool,
	version string,
	userspaceMode bool,
	stateStore state.Store,
	stateDir string,
	ctx context.Context,
	orgId string,
) (*Nexodus, error) {

	if err := binaryChecks(); err != nil {
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
		wireguardPubKey:         wireguardPubKey,
		wireguardPvtKey:         wireguardPvtKey,
		listenPort:              wgListenPort,
		requestedIP:             requestedIP,
		userProvidedLocalIP:     userProvidedLocalIP,
		childPrefix:             childPrefix,
		stun:                    stun,
		relay:                   relay,
		networkRouter:           networkRouterNode,
		networkRouterDisableNAT: networkRouterDisableNAT,
		deviceCache:             make(map[string]deviceCacheEntry),
		apiURL:                  apiURL,
		hostname:                hostname,
		symmetricNat:            relayOnly,
		logger:                  logger,
		logLevel:                logLevel,
		status:                  NexdStatusStarting,
		version:                 version,
		username:                username,
		password:                password,
		skipTlsVerify:           insecureSkipTlsVerify,
		stateStore:              stateStore,
		orgId:                   orgId,
		userspaceWG: userspaceWG{
			proxies: map[ProxyKey]*UsProxy{},
		},
	}
	ax.userspaceMode = userspaceMode
	ax.tunnelIface = ax.defaultTunnelDev()

	if ax.relay {
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

	if err := ax.symmetricNatDisco(ctx); err != nil {
		ax.logger.Warn(err)
	}

	err = ax.migrateLegacyState(stateDir)
	if err != nil {
		return nil, err
	}

	return ax, nil
}

func (nx *Nexodus) SetStatus(status int, msg string) {
	nx.statusMsg = msg
	nx.status = status
}

type StateTokenStore struct {
	store state.Store
}

func (s StateTokenStore) Load() (*oauth2.Token, error) {
	err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return s.store.State().AuthToken, nil
}

func (s StateTokenStore) Store(token *oauth2.Token) error {
	s.store.State().AuthToken = token
	return s.store.Store()
}

var _ client.TokenStore = StateTokenStore{}

func (nx *Nexodus) migrateLegacyState(stateDir string) error {
	err := nx.stateStore.Load()
	if err != nil {
		return err
	}

	s := nx.stateStore.State()
	if s.PublicKey == "" || s.PrivateKey == "" {
		// We used to store the keys in a different location
		// migrate them to the state store
		s.PrivateKey, s.PublicKey, err = nx.loadLegacyKeys()
		if err != nil {
			return err
		}
	}

	if s.AuthToken == nil {
		legacyApitokenFile := filepath.Join(stateDir, "apitoken.json")
		if _, err = os.Stat(legacyApitokenFile); err == nil {
			data, err := os.ReadFile(legacyApitokenFile)
			if err != nil {
				return err
			}
			token := oauth2.Token{}
			err = json.Unmarshal(data, &token)
			if err != nil {
				return err
			}
			s.AuthToken = &token
			_ = os.Remove(legacyApitokenFile)
		}
	}

	legacyRulesFile := filepath.Join(stateDir, "proxy-rules.json")
	if _, err = os.Stat(legacyRulesFile); err == nil {
		data, err := os.ReadFile(legacyRulesFile)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, &s.ProxyRulesConfig)
		if err != nil {
			return err
		}
		_ = os.Remove(legacyRulesFile)
	}

	return nx.stateStore.Store()
}

func (nx *Nexodus) Start(ctx context.Context, wg *sync.WaitGroup) error {
	nx.nexCtx = ctx
	nx.nexWg = wg

	// Block additional proxy configuration coming in via the ctl server until after
	// initial startup is complete.
	nx.proxyLock.Lock()
	defer nx.proxyLock.Unlock()

	if err := nx.CtlServerStart(ctx, wg); err != nil {
		return fmt.Errorf("CtlServerStart(): %w", err)
	}

	if runtime.GOOS != Linux.String() {
		nx.logger.Info("Security Groups are currently only supported on Linux")
	} else if nx.userspaceMode {
		nx.logger.Info("Security Groups are not supported in userspace proxy mode")
	}

	var options []client.Option
	if nx.stateStore != nil {
		options = append(options, client.WithTokenStore(StateTokenStore{store: nx.stateStore}))
	}
	if nx.username == "" {
		options = append(options, client.WithDeviceFlow())
	} else if nx.username != "" && nx.password == "" {
		fmt.Print("Enter nexodus account password: ")
		passwdInput, err := term.ReadPassword(int(syscall.Stdin))
		println()
		if err != nil {
			return fmt.Errorf("login aborted: %w", err)
		}
		nx.password = string(passwdInput)
		options = append(options, client.WithPasswordGrant(nx.username, nx.password))
	} else {
		options = append(options, client.WithPasswordGrant(nx.username, nx.password))
	}
	if nx.skipTlsVerify { // #nosec G402
		options = append(options, client.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}

	var err error
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		nx.client, err = client.NewAPIClient(ctx, nx.apiURL.String(), func(msg string) {
			nx.SetStatus(NexdStatusAuth, msg)
		}, options...)
		if err != nil {
			nx.logger.Warnf("client api error - retrying: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("client api error: %w", err)
	}

	nx.SetStatus(NexdStatusRunning, "")

	if err := nx.handleKeys(); err != nil {
		return fmt.Errorf("handleKeys: %w", err)
	}

	var user *public.ModelsUser
	var resp *http.Response
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		user, resp, err = nx.client.UsersApi.GetUser(ctx, "me").Execute()
		if err != nil {
			if resp != nil {
				nx.logger.Warnf("get user error - retrying error: %v header: %+v", err, resp.Header)
				return err
			} else if strings.Contains(err.Error(), invalidTokenGrant.Error()) || strings.Contains(err.Error(), invalidToken.Error()) {
				nx.logger.Errorf("The nexodus token stored in %s is not valid for the api-server, you can remove the file and try again: %v", nx.stateStore, err)
				return err
			} else {
				nx.logger.Warnf("get user error - retrying error: %v", err)
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("get user error: %w", err)
	}

	var organizations []public.ModelsOrganization
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		organizations, resp, err = nx.client.OrganizationsApi.ListOrganizations(ctx).Execute()
		if err != nil {
			if resp != nil {
				nx.logger.Warnf("get organizations error - retrying error: %v header: %+v", err, resp.Header)
				return err
			}
			if err != nil {
				nx.logger.Warnf("get organizations error - retrying error: %v", err)
				return err
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("get organizations error: %w", err)
	}

	nx.org, err = nx.chooseOrganization(organizations, *user)
	if err != nil {
		return fmt.Errorf("failed to choose an organization: %w", err)
	}

	informerCtx, informerCancel := context.WithCancel(ctx)
	nx.informerStop = informerCancel
	nx.informer = nx.client.DevicesApi.ListDevicesInOrganization(informerCtx, nx.org.Id).Informer()

	var localIP string
	var localEndpointPort int

	// User requested ip --request-ip takes precedent
	if nx.userProvidedLocalIP != "" {
		localIP = nx.userProvidedLocalIP
		localEndpointPort = nx.listenPort
	}

	if nx.relay {
		peerMap, _, err := nx.informer.Execute()
		if err != nil {
			return err
		}

		existingRelay, err := nx.orgRelayCheck(peerMap)
		if err != nil {
			return err
		}
		if existingRelay != "" {
			return fmt.Errorf("the organization already contains a relay node, device %s needs to be deleted before adding a new relay", existingRelay)
		}
	}

	// If we are behind a symmetricNat, the endpoint ip discovered by a stun server is useless
	stunServer1 := stun.NextServer()
	if !nx.symmetricNat && nx.stun && localIP == "" {
		ipPort, err := stun.Request(nx.logger, stunServer1, nx.listenPort)
		if err != nil {
			nx.logger.Warn("Unable to determine the public facing address, falling back to the local address")
		} else {
			localIP = ipPort.Addr().String()
			localEndpointPort = int(ipPort.Port())
		}
	}
	if localIP == "" {
		ip, err := nx.findLocalIP()
		if err != nil {
			return fmt.Errorf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %w", err)
		}
		localIP = ip
		localEndpointPort = nx.listenPort
	}

	nx.os = runtime.GOOS

	nx.endpointLocalAddress = localIP

	// if this device is a network router node, enable ip forwarding and set up the network router netfilter policy
	if nx.networkRouter {
		err := nx.setupNetworkRouterNode()
		if err != nil {
			return fmt.Errorf("failed to setup this device as a network router node: %w", err)
		}
	}

	endpointSocket := net.JoinHostPort(localIP, fmt.Sprintf("%d", localEndpointPort))
	endpoints := []public.ModelsEndpoint{
		{
			Source:   "local",
			Address:  endpointSocket,
			Distance: 0,
		},
		{
			Source:   "stun:" + stunServer1,
			Address:  nx.nodeReflexiveAddressIPv4.String(),
			Distance: 0,
		},
	}

	var modelsDevice public.ModelsDevice
	var deviceOperationLogMsg string
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		modelsDevice, deviceOperationLogMsg, err = nx.createOrUpdateDeviceOperation(user.Id, endpoints)
		if err != nil {
			nx.logger.Warnf("device join error - retrying: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("join error %w", err)
	}
	nx.logger.Debug(fmt.Sprintf("Device: %+v", modelsDevice))
	nx.logger.Infof("%s with UUID: [ %+v ] into organization: [ %s (%s) ]",
		deviceOperationLogMsg, modelsDevice.Id, nx.org.Name, nx.org.Id)

	// a relay node requires ip forwarding and nftable rules, OS type has already been checked
	if nx.relay {
		if err := nx.enableForwardingIP(); err != nil {
			return err
		}
		if err := nfRelayTablesSetup(wgIface); err != nil {
			return err
		}
	}

	util.GoWithWaitGroup(wg, func() {
		// kick it off with an immediate reconcile
		nx.reconcileDevices(ctx, options)
		nx.reconcileSecurityGroups(ctx)
		for _, proxy := range nx.proxies {
			proxy.Start(ctx, wg, nx.userspaceNet)
		}
		stunTicker := time.NewTicker(time.Second * 20)
		secGroupTicker := time.NewTicker(time.Second * 20)
		defer stunTicker.Stop()
		pollTicker := time.NewTicker(pollInterval)
		defer pollTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stunTicker.C:
				if err := nx.reconcileStun(modelsDevice.Id); err != nil {
					nx.logger.Debug(err)
				}
			case <-nx.informer.Changed():
				nx.reconcileDevices(ctx, options)
			case <-pollTicker.C:
				// This does not actually poll the API for changes. Peer configuration changes will only
				// be processed when they come in on the informer. This periodic check is needed to
				// re-establish our connection to the API if it is lost.
				nx.reconcileDevices(ctx, options)
			case <-secGroupTicker.C:
				nx.reconcileSecurityGroups(ctx)
			}
		}
	})

	return nil
}

func (nx *Nexodus) Stop() {
	nx.logger.Info("Stopping nexd")
	for _, proxy := range nx.proxies {
		proxy.Stop()
	}
}

// reconcileSecurityGroups will check the security group and update it if necessary.
func (nx *Nexodus) reconcileSecurityGroups(ctx context.Context) {
	if runtime.GOOS != Linux.String() || nx.userspaceMode {
		return
	}

	existing, ok := nx.deviceCacheLookup(nx.wireguardPubKey)
	if !ok {
		// local device not in the cache, so we don't have our config yet.
		return
	}

	if existing.device.SecurityGroupId == uuid.Nil.String() {
		// local device has no security group
		if nx.securityGroup == nil {
			// already set up that way, nothing to do
			return
		}
		// drop local security group configuration
		nx.securityGroup = nil
		if err := nx.processSecurityGroupRules(); err != nil {
			nx.logger.Error(err)
		}
		return
	}

	// if the security group ID is not nil, lookup the ID and check for any changes
	responseSecGroup, httpResp, err := nx.client.SecurityGroupApi.GetSecurityGroup(ctx, nx.org.Id, existing.device.SecurityGroupId).Execute()
	if err != nil {
		// if the group ID returns a 404, clear the current rules
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			nx.securityGroup = nil
			if err := nx.processSecurityGroupRules(); err != nil {
				nx.logger.Error(err)
			}
			return
		}
		nx.logger.Errorf("Error retrieving the security group: %v", err)
		return
	}

	if nx.securityGroup != nil && reflect.DeepEqual(responseSecGroup, nx.securityGroup) {
		// no changes to previously applied security group
		return
	}

	nx.logger.Debugf("Security Group change detected: %+v", responseSecGroup)
	oldSecGroup := nx.securityGroup
	nx.securityGroup = responseSecGroup

	if oldSecGroup != nil && responseSecGroup.Id == oldSecGroup.Id &&
		reflect.DeepEqual(responseSecGroup.InboundRules, oldSecGroup.InboundRules) &&
		reflect.DeepEqual(responseSecGroup.OutboundRules, oldSecGroup.OutboundRules) {
		// the group changed, but not in a way that matters for applying the rules locally
		return
	}

	// apply the new security group rules
	if err := nx.processSecurityGroupRules(); err != nil {
		nx.logger.Error(err)
	}
}

func (nx *Nexodus) reconcileDevices(ctx context.Context, options []client.Option) {
	var err error
	if err = nx.reconcileDeviceCache(); err == nil {
		return
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.Temporary() {
		// Temporary dns resolution failure is normal, just debug log it
		nx.logger.Debugf("%v", err)
		return
	}

	nx.logger.Errorf("Failed to reconcile state with the nexodus API server: %v", err)

	// if the token grant becomes invalid expires refresh or exit depending on the onboard method
	if !strings.Contains(err.Error(), invalidTokenGrant.Error()) {
		return
	}

	// token grant has become invalid, if we are using a one-time auth token, exit
	if nx.username == "" {
		nx.logger.Fatalf("The token grant has expired due to an extended period offline, please " +
			"restart the agent for a one-time auth or login with --username --password to automatically reconnect")
		return
	}

	// do we need to stop the informer?
	if nx.informerStop != nil {
		nx.informerStop()
		nx.informerStop = nil
	}

	// refresh the token grant by reconnecting to the API server
	c, err := client.NewAPIClient(ctx, nx.apiURL.String(), func(msg string) {
		nx.SetStatus(NexdStatusAuth, msg)
	}, options...)
	if err != nil {
		nx.logger.Errorf("Failed to reconnect to the api-server, retrying in %v seconds: %v", pollInterval, err)
		return
	}

	nx.client = c
	informerCtx, informerCancel := context.WithCancel(ctx)
	nx.informerStop = informerCancel
	nx.informer = nx.client.DevicesApi.ListDevicesInOrganization(informerCtx, nx.org.Id).Informer()

	nx.SetStatus(NexdStatusRunning, "")
	nx.logger.Infoln("Nexodus agent has re-established a connection to the api-server")
}

func (nx *Nexodus) reconcileStun(deviceID string) error {
	if nx.symmetricNat {
		return nil
	}

	nx.logger.Debug("sending stun request")
	stunServer1 := stun.NextServer()
	reflexiveIP, err := stun.Request(nx.logger, stunServer1, nx.listenPort)
	if err != nil {
		return fmt.Errorf("stun request error: %w", err)
	}

	if nx.nodeReflexiveAddressIPv4 != reflexiveIP {
		nx.logger.Infof("detected a NAT binding changed for this device %s from %s to %s, updating peers", deviceID, nx.nodeReflexiveAddressIPv4, reflexiveIP)

		res, _, err := nx.client.DevicesApi.UpdateDevice(context.Background(), deviceID).Update(public.ModelsUpdateDevice{
			Endpoints: []public.ModelsEndpoint{
				{
					Source:   "local",
					Address:  net.JoinHostPort(nx.endpointLocalAddress, fmt.Sprintf("%d", nx.listenPort)),
					Distance: 0,
				},
				{
					Source:   "stun:" + stunServer1,
					Address:  reflexiveIP.String(),
					Distance: 0,
				},
			},
		}).Execute()
		if err != nil {
			return fmt.Errorf("failed to update this device's new NAT binding, likely still reconnecting to the api-server, retrying in 20s: %w", err)
		} else {
			nx.logger.Debugf("update device response %+v", res)
			nx.nodeReflexiveAddressIPv4 = reflexiveIP
			// reinitialize peers if the NAT binding has changed for the node
			if err = nx.reconcileDeviceCache(); err != nil {
				nx.logger.Debugf("reconcile failed %v", res)
			}
		}
	}

	return nil
}

func (nx *Nexodus) chooseOrganization(organizations []public.ModelsOrganization, user public.ModelsUser) (*public.ModelsOrganization, error) {
	if len(organizations) == 0 {
		return nil, fmt.Errorf("user does not belong to any organizations")
	}
	if nx.orgId == "" {
		if len(organizations) > 1 {
			// default to the org that matches the user name, the one created for a new user by default
			for i, org := range organizations {
				if org.Name == user.UserName {
					return &organizations[i], nil
				}
			}
			// Log all org names + Ids for convenience before returning the error
			for _, org := range organizations {
				nx.logger.Infof("organization name: '%s'  Id: %s", org.Name, org.Id)
			}
			return nil, fmt.Errorf("user belongs to multiple organizations, please specify one with --organization-id")
		}
		return &organizations[0], nil
	}
	for i, org := range organizations {
		if org.Id == nx.orgId {
			return &organizations[i], nil
		}
	}
	return nil, fmt.Errorf("user does not belong to organization %s", nx.orgId)
}

func (nx *Nexodus) deviceCacheIterRead(f func(deviceCacheEntry)) {
	nx.deviceCacheLock.RLock()
	defer nx.deviceCacheLock.RUnlock()

	for _, d := range nx.deviceCache {
		f(d)
	}
}

func (nx *Nexodus) deviceCacheLookup(pubKey string) (deviceCacheEntry, bool) {
	nx.deviceCacheLock.RLock()
	defer nx.deviceCacheLock.RUnlock()

	d, ok := nx.deviceCache[pubKey]
	return d, ok
}

func (nx *Nexodus) peerIsHealthy(d deviceCacheEntry) bool {
	// How do we determine a peer is healthy or not?
	//
	// We have wireguard keepalives turned on, so if we haven't seen
	// bidirectional traffic in the keepalive interval + the keepalive timeout,
	// then it is unhealthy. We can watch the tx and rx counters for this.
	// We can also watch the LastHandshakeTime.
	//
	// The next method we could use here is to check the LastHandshakeTime.
	// If it is older than the RekeyAfterTime + RekeyTimeout, then it is unhealthy.
	// That would result in a slower detection of an unhealthy peer than watching
	// for counters to increment based on keepalives based on our current keepalive
	// setting, though.

	if d.lastHandshakeTime.IsZero() {
		// We haven't seen a handshake yet, so this peer connection is not up.
		if d.peerHealthy {
			nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is unhealthy due to no handshake",
				d.device.Hostname, d.device.PublicKey,
				d.device.Endpoints[0].Address, d.device.Endpoints[1].Address)
		}
		return false
	}

	keepaliveWindow := keepaliveInterval + device.KeepaliveTimeout

	if time.Since(d.lastHandshakeTime) < keepaliveWindow {
		// We have seen a handshake recently enough, so this peer connection is up.
		if !d.peerHealthy {
			nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is now healthy due to lastHandshakeTime: %s < %s",
				d.device.Hostname, d.device.PublicKey,
				d.device.Endpoints[0].Address, d.device.Endpoints[1].Address,
				time.Since(d.lastHandshakeTime).String(), keepaliveWindow.String())
		}
		return true
	}

	if time.Since(d.startTime) < keepaliveWindow {
		// We haven't been tracking this peer long enough to know if it is healthy or not,
		// so assume the best.
		if !d.peerHealthy {
			nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is assumed healthy due to startTime: %s < %s",
				d.device.Hostname, d.device.PublicKey,
				d.device.Endpoints[0].Address, d.device.Endpoints[1].Address,
				time.Since(d.startTime).String(), keepaliveWindow.String())
		}
		return true
	}

	if time.Since(d.lastTxTime) > keepaliveWindow {
		if d.peerHealthy {
			nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is unhealthy due to lastTxTime: %s",
				d.device.Hostname, d.device.PublicKey,
				d.device.Endpoints[0].Address, d.device.Endpoints[1].Address,
				time.Since(d.lastTxTime).String())
		}
		return false
	}

	if time.Since(d.lastRxTime) > keepaliveWindow {
		if d.peerHealthy {
			nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is unhealthy due to lastRxTime: %s",
				d.device.Hostname, d.device.PublicKey,
				d.device.Endpoints[0].Address, d.device.Endpoints[1].Address,
				time.Since(d.lastRxTime).String())
		}
		return false
	}

	if !d.peerHealthy {
		nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is now healthy based on tx/rx counter activity",
			d.device.Hostname, d.device.PublicKey,
			d.device.Endpoints[0].Address, d.device.Endpoints[1].Address)
	}

	return true
}

// assumes deviceCacheLock is held with a write-lock
func (nx *Nexodus) addToDeviceCache(p public.ModelsDevice) {
	nx.deviceCache[p.PublicKey] = deviceCacheEntry{
		device:      p,
		lastUpdated: time.Now(),
	}
}

func (nx *Nexodus) reconcileDeviceCache() error {
	peerMap, resp, err := nx.informer.Execute()
	if err != nil {
		if resp != nil {
			return fmt.Errorf("error: %w header: %v", err, resp.Header)
		}
		return fmt.Errorf("error: %w", err)
	}

	// Get the current peer configuration data from the wireguard interface
	peerStats, err := nx.DumpPeersDefault()
	if err != nil {
		if nx.TunnelIP != "" {
			// Unexpected to fail once we have local interface configuration
			nx.logger.Warnf("failed to get current peer stats: %w", err)
		}
		peerStats = make(map[string]WgSessions)
	}

	nx.deviceCacheLock.Lock()
	defer nx.deviceCacheLock.Unlock()

	// Get our device cache up to date
	newLocalConfig := false
	for _, p := range peerMap {
		// Update the cache if the device is new or has changed
		existing, ok := nx.deviceCache[p.PublicKey]
		if !ok || !nx.isEqualIgnoreSecurityGroup(existing.device, p) {
			if p.PublicKey == nx.wireguardPubKey {
				newLocalConfig = true
			}
			nx.addToDeviceCache(p)
			existing = nx.deviceCache[p.PublicKey]
		}

		// Store the relay IP for easy reference later
		if p.Relay {
			nx.relayWgIP = p.AllowedIps[0]
		}

		// Keep track of peer connection stats for connection health tracking
		curStats, ok := peerStats[p.PublicKey]
		if !ok {
			// This won't be available early because the peer hasn't been configured yet
			continue
		}
		if curStats.Tx != existing.lastTxBytes {
			existing.lastTxBytes = curStats.Tx
			existing.lastTxTime = time.Now()
		}
		if curStats.Rx != existing.lastRxBytes {
			existing.lastRxBytes = curStats.Rx
			existing.lastRxTime = time.Now()
		}
		existing.lastHandshakeTime = curStats.LastHandshakeTime
		existing.lastHandshake = curStats.LatestHandshake
		existing.lastRefresh = time.Now()
		existing.endpoint = curStats.Endpoint
		existing.peerHealthy = nx.peerIsHealthy(existing)
		nx.deviceCache[p.PublicKey] = existing
	}

	// Refresh wireguard peer configuration, getting any new peers or changes to existing peers
	updatePeers := nx.buildPeersConfig()
	if newLocalConfig || len(updatePeers) > 0 {
		// Reset connection health tracking data for any peers that have changed
		for _, peer := range updatePeers {
			existing, ok := nx.deviceCache[peer.PublicKey]
			if !ok {
				continue
			}
			nx.logger.Debugf("resetting connection health tracking for peer %s", peer.PublicKey)
			existing.startTime = time.Now()
			if peerConfig, ok := nx.wgConfig.Peers[peer.PublicKey]; ok {
				existing.endpoint = peerConfig.Endpoint
			}
			nx.deviceCache[peer.PublicKey] = existing
		}

		// Deploy updated wireguard peer configuration
		if err := nx.DeployWireguardConfig(updatePeers); err != nil {
			if strings.Contains(err.Error(), securityGroupErr.Error()) {
				return err
			}
			// If the wireguard configuration fails, we should wipe out our peer list
			// so it is rebuilt and reconfigured from scratch the next time around.
			nx.wgConfig.Peers = nil
			return err
		}
	}

	// check for any peer deletions
	if err := nx.handlePeerDelete(peerMap); err != nil {
		nx.logger.Error(err)
	}

	return nil
}

func (nx *Nexodus) isEqualIgnoreSecurityGroup(p1, p2 public.ModelsDevice) bool {
	// create temporary copies of the instances
	tmpDev1 := p1
	tmpDev2 := p2
	// set the SecurityGroupId to an empty value, so it will not affect the comparison
	tmpDev1.SecurityGroupId = ""
	tmpDev2.SecurityGroupId = ""
	// set the Revision to 0, so it will not affect the comparison
	tmpDev1.Revision = 0
	tmpDev2.Revision = 0

	return reflect.DeepEqual(tmpDev1, tmpDev2)
}

// checkUnsupportedConfigs general matrix checks of required information or constraints to run the agent and join the mesh
func (nx *Nexodus) checkUnsupportedConfigs() error {

	if nx.ipv6Supported = isIPv6Supported(); !nx.ipv6Supported {
		nx.logger.Warn("IPv6 does not appear to be enabled on this host, only IPv4 will be provisioned or restart nexd with IPv6 enabled on this host")
	}

	return nil
}

// symmetricNatDisco determine if the joining node is within a symmetric NAT cone
func (nx *Nexodus) symmetricNatDisco(ctx context.Context) error {

	stunRetryTimer := time.Second * 1
	err := util.RetryOperation(ctx, stunRetryTimer, maxRetries, func() error {
		stunServer1 := stun.NextServer()
		stunServer2 := stun.NextServer()
		stunAddr1, err := stun.Request(nx.logger, stunServer1, nx.listenPort)
		if err != nil {
			return err
		} else {
			nx.nodeReflexiveAddressIPv4 = stunAddr1
		}

		isSymmetric := false
		stunAddr2, err := stun.Request(nx.logger, stunServer2, nx.listenPort)
		if err != nil {
			return err
		} else {
			isSymmetric = stunAddr1.String() != stunAddr2.String()
		}

		if stunAddr1.Addr().String() != "" {
			nx.logger.Debugf("first NAT discovery STUN request returned: %s", stunAddr1.String())
		} else {
			nx.logger.Debugf("first NAT discovery STUN request returned an empty value")
		}

		if stunAddr2.Addr().String() != "" {
			nx.logger.Debugf("second NAT discovery STUN request returned: %s", stunAddr2.String())
		} else {
			nx.logger.Debugf("second NAT discovery STUN request returned an empty value")
		}

		if isSymmetric {
			nx.symmetricNat = true
			nx.logger.Infof("Symmetric NAT is detected, this node will be provisioned in relay mode only")
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("STUN discovery error: %w", err)
	}

	return nil
}

// orgRelayCheck checks if there is an existing Relay node in the organization that does not match this device's pub key
func (nx *Nexodus) orgRelayCheck(peerMap map[string]public.ModelsDevice) (string, error) {
	for _, p := range peerMap {
		if p.Relay && nx.wireguardPubKey != p.PublicKey {
			return p.Id, nil
		}
	}
	return "", nil
}

func (nx *Nexodus) setupInterface() error {
	if nx.userspaceMode {
		return nx.setupInterfaceUS()
	}
	return nx.setupInterfaceOS()
}

func (nx *Nexodus) defaultTunnelDev() string {
	if nx.userspaceMode {
		return nx.defaultTunnelDevUS()
	}
	return defaultTunnelDevOS()
}
