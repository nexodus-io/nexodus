package nexodus

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/golang-jwt/jwt/v4"
	"github.com/nexodus-io/nexodus/internal/wgcrypto"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/nexodus-io/nexodus/internal/state"
	"golang.org/x/oauth2"

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
	darwinIface        = "utun8"
	WgDefaultPort      = 51820
	wgIface            = "wg0"
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
	// the last time we saw this peer as healthy
	peerHealthyTime time.Time
}

type deviceCacheEntry struct {
	device public.ModelsDevice
	// the last time this device was updated as seen from the API
	lastUpdated time.Time
	peerHealth
	peeringMethod      string
	peeringMethodIndex int
	// The last time a new peering configuration was generated for this device
	peeringTime time.Time
}

type exitNode struct {
	exitNodeExists        bool
	exitNodeClientEnabled bool
	exitNodeOriginEnabled bool
	exitNodeOrigins       []wgPeerConfig
}

type Nexodus struct {
	wireguardPubKey          string
	wireguardPvtKey          string
	wireguardPubKeyInConfig  bool
	tunnelIface              string
	listenPort               int
	vpcId                    string
	vpc                      *public.ModelsVPC
	requestedIP              string
	userProvidedLocalIP      string
	TunnelIP                 string
	TunnelIpV6               string
	advertiseCidrs           []string
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
	reflexiveAddrStunSrc     string
	hostname                 string
	securityGroup            *public.ModelsSecurityGroup
	symmetricNat             bool
	ipv6Supported            bool
	deviceReconciled         bool
	os                       string
	exitNode                 exitNode
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
	stateDir      string
	userspaceWG
	securityGroupsInformer *public.Informer[public.ModelsSecurityGroup]
	devicesInformer        *public.Informer[public.ModelsDevice]
	informerStop           context.CancelFunc
	nexCtx                 context.Context
	nexWg                  *sync.WaitGroup
	registrationToken      string
	clientOptions          []client.Option
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

func NewNexodus(logger *zap.SugaredLogger, logLevel *zap.AtomicLevel, apiURL *url.URL, registrationToken string, username string, password string, wgListenPort int, requestedIP string, userProvidedLocalIP string, advertiseCidrs []string, relay bool, relayOnly bool, networkRouterNode bool, networkRouterDisableNAT bool, exitNodeClientEnabled bool, exitNodeOriginEnabled bool, insecureSkipTlsVerify bool, version string, userspaceMode bool, stateStore state.Store, stateDir string, ctx context.Context, vpcId string) (*Nexodus, error) {
	public.Logger = logger
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

	nx := &Nexodus{
		listenPort:              wgListenPort,
		requestedIP:             requestedIP,
		userProvidedLocalIP:     userProvidedLocalIP,
		advertiseCidrs:          advertiseCidrs,
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
		registrationToken:       registrationToken,
		username:                username,
		password:                password,
		skipTlsVerify:           insecureSkipTlsVerify,
		stateStore:              stateStore,
		stateDir:                stateDir,
		vpcId:                   vpcId,
		userspaceWG: userspaceWG{
			proxies: map[ProxyKey]*UsProxy{},
		},
		exitNode: exitNode{
			exitNodeClientEnabled: exitNodeClientEnabled,
			exitNodeOriginEnabled: exitNodeOriginEnabled,
		},
	}

	nx.userspaceMode = userspaceMode

	if !nx.userspaceMode {
		isOk, err := isElevated()
		if !isOk {
			return nil, err
		}
	}

	nx.tunnelIface = nx.defaultTunnelDev()

	if nx.relay {
		nx.listenPort = WgDefaultPort
	}

	if err := nx.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	// remove orphaned wg interfaces from previous node joins
	nx.removeExistingInterface()

	if err := nx.symmetricNatDisco(ctx); err != nil {
		nx.logger.Warn(err)
	}

	err = nx.migrateLegacyState(stateDir)
	if err != nil {
		return nil, err
	}

	return nx, nil
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

func (nx *Nexodus) resetApiClient(ctx context.Context) error {
	var err error
	nx.client, err = client.NewAPIClient(ctx, nx.apiURL.String(), func(msg string) {
		nx.SetStatus(NexdStatusAuth, msg)
	}, nx.clientOptions...)
	if err != nil {
		nx.logger.Warnf("client api error - retrying: %v", err)
		return err
	}
	return nil
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

	if runtime.GOOS != Linux.String() && runtime.GOOS != Darwin.String() {
		nx.logger.Info("Security Groups are currently only supported on Linux and macOS")
	} else if nx.userspaceMode {
		nx.logger.Info("Security Groups are not supported in userspace proxy mode")
	}

	options := []client.Option{
		client.WithUserAgent(fmt.Sprintf("nexd/%s (%s; %s)", nx.version, runtime.GOOS, runtime.GOARCH)),
	}
	if nx.registrationToken != "" {
		// the reg token can be used to get the device token
		options = append(options, client.WithBearerToken(nx.registrationToken))
	} else {
		// fallback to using oauth flows to get the device token... these are either interactive or
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
	}
	if nx.skipTlsVerify { // #nosec G402
		options = append(options, client.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}
	nx.clientOptions = options

	var err error
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		return nx.resetApiClient(ctx)
	})
	if err != nil {
		return fmt.Errorf("client api error: %w", err)
	}

	nx.SetStatus(NexdStatusRunning, "")

	if err := nx.handleKeys(); err != nil {
		return fmt.Errorf("handleKeys: %w", err)
	}
	userId, org, err := nx.fetchUserIdAndVpc(ctx)
	if err != nil {
		return err
	}

	nx.vpc = org

	// User requested ip --request-ip takes precedent
	if nx.userProvidedLocalIP != "" {
		nx.endpointLocalAddress = nx.userProvidedLocalIP
	} else {
		nx.endpointLocalAddress, err = nx.findLocalIP()
		if err != nil {
			return fmt.Errorf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %w", err)
		}
	}

	nx.os = runtime.GOOS

	// if this device is a network router node, enable ip forwarding and set up the network router netfilter policy
	if nx.networkRouter {
		err := nx.setupNetworkRouterNode()
		if err != nil {
			return fmt.Errorf("failed to setup this device as a network router node: %w", err)
		}
	}

	if nx.exitNode.exitNodeOriginEnabled {
		if err := nx.exitNodeOriginSetup(); err != nil {
			return fmt.Errorf("failed to setup this device as an exit-node: %w", err)
		}
	}

	endpointSocket := net.JoinHostPort(nx.endpointLocalAddress, fmt.Sprintf("%d", nx.listenPort))
	endpoints := []public.ModelsEndpoint{
		{
			Source:   "local",
			Address:  endpointSocket,
			Distance: 0,
		},
		{
			Source:   "stun:" + nx.reflexiveAddrStunSrc,
			Address:  nx.nodeReflexiveAddressIPv4.String(),
			Distance: 0,
		},
	}

	var modelsDevice public.ModelsDevice
	var deviceOperationLogMsg string
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		modelsDevice, deviceOperationLogMsg, err = nx.createOrUpdateDeviceOperation(userId, endpoints)
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
	nx.logger.Infof("%s with UUID: [ %+v ] into vpc: [ %s (%s) ]",
		deviceOperationLogMsg, modelsDevice.Id, nx.vpc.Id, nx.vpc.Description)

	// Use the device token to auth with the apiserver...
	if modelsDevice.BearerToken != "" {

		key, err := wgtypes.ParseKey(nx.wireguardPvtKey)
		if err != nil {
			return err
		}

		sealed, err := wgcrypto.ParseSealed(modelsDevice.BearerToken)
		if err != nil {
			return err
		}

		data, err := sealed.Open(key[:])
		if err != nil {
			return err
		}

		//nx.stateStore.State().DeviceToken = string(data)
		//err = nx.stateStore.Store()
		//if err != nil {
		//	return err
		//}

		options = append(options, client.WithBearerToken(string(data)))
		nx.client, err = client.NewAPIClient(ctx, nx.apiURL.String(), func(msg string) {}, options...)
		if err != nil {
			return err
		}
	}

	informerCtx, informerCancel := context.WithCancel(ctx)
	nx.informerStop = informerCancel

	// event stream sharing occurs due to the informers sharing the context created in following line:
	informerCtx = nx.client.VPCApi.WatchEvents(informerCtx, nx.vpc.Id).PublicKey(nx.wireguardPubKey).NewSharedInformerContext()
	nx.securityGroupsInformer = nx.client.VPCApi.ListSecurityGroupsInVPC(informerCtx, nx.vpc.Id).Informer()
	nx.devicesInformer = nx.client.VPCApi.ListDevicesInVPC(informerCtx, nx.vpc.Id).Informer()

	if nx.relay {
		peerMap, _, err := nx.devicesInformer.Execute()
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
		if nx.exitNode.exitNodeClientEnabled {
			if err := nx.ExitNodeClientSetup(); err != nil {
				nx.logger.Errorf("failed to enable this device as an exit-node client: %v", err)
			}
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
					if nx.os != Windows.String() { // windows does not currently support reuse port or bpf
						nx.logger.Debug(err)
					}
				}
			case <-nx.devicesInformer.Changed():
				nx.reconcileDevices(ctx, options)
			case <-nx.securityGroupsInformer.Changed():
				nx.reconcileSecurityGroups(ctx)
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

type NexodusClaims struct {
	jwt.RegisteredClaims
	Scope          string    `json:"scope,omitempty"`
	OrganizationID uuid.UUID `json:"org,omitempty"`
	DeviceID       uuid.UUID `json:"device,omitempty"`
}

func (nx *Nexodus) fetchUserIdAndVpc(ctx context.Context) (string, *public.ModelsVPC, error) {
	if nx.registrationToken != "" {
		// the userid and orgid are part of the registration token.
		return nx.fetchRegistrationTokenUserIdAndVPC(ctx)
	} else {
		// Use the API to figure out the user's id and org
		return nx.fetchUserIdAndVpcFromAPI(ctx)
	}
}

func (nx *Nexodus) fetchRegistrationTokenUserIdAndVPC(ctx context.Context) (string, *public.ModelsVPC, error) {

	// get the certs used to validate the JWT.
	regToken, _, err := nx.client.RegistrationTokenApi.GetRegistrationToken(ctx, "me").Execute()
	if err != nil {
		return "", nil, fmt.Errorf("could not fetch registration settings: %w", err)
	}

	vpc, _, err := nx.client.VPCApi.GetVPC(ctx, regToken.VpcId).Execute()
	if err != nil {
		return "", nil, err
	}
	return regToken.OwnerId, vpc, nil
}

func (nx *Nexodus) fetchUserIdAndVpcFromAPI(ctx context.Context) (string, *public.ModelsVPC, error) {

	var err error
	var user *public.ModelsUser
	var resp *http.Response
	err = util.RetryOperationExpBackoff(ctx, retryInterval, func() error {
		user, resp, err = nx.client.UsersApi.GetUser(ctx, "me").Execute()
		if err != nil {
			if strings.Contains(err.Error(), invalidTokenGrant.Error()) || strings.Contains(err.Error(), invalidToken.Error()) ||
				strings.Contains(resp.Header.Get("Www-Authenticate"), invalidToken.Error()) {
				nx.logger.Debug("invalid auth token, removing and retrying")
				s := nx.stateStore.State()
				s.AuthToken = nil
				_ = nx.stateStore.Store()
				_ = nx.resetApiClient(ctx)
				nx.SetStatus(NexdStatusRunning, "")
				return err
			} else if resp != nil {
				nx.logger.Warnf("get user error - retrying error: %v header: %+v", err, resp.Header)
				return err
			} else {
				nx.logger.Warnf("get user error - retrying error: %v", err)
				return err
			}
		}
		return nil
	})

	if err != nil {
		return "", nil, fmt.Errorf("get user error: %w", err)
	}

	var vpc *public.ModelsVPC
	err = util.RetryOperation(ctx, retryInterval, maxRetries, func() error {
		if nx.vpcId == "" {
			nx.vpcId = user.Id
		}

		vpc, resp, err = nx.client.VPCApi.GetVPC(ctx, nx.vpcId).Execute()
		if err != nil {
			if resp != nil {
				nx.logger.Warnf("get vpc error - retrying error: %v header: %+v", err, resp.Header)
				return err
			}
			if err != nil {
				nx.logger.Warnf("get vpc error - retrying error: %v", err)
				return err
			}
		}

		return nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("get vpc error: %w", err)
	}

	return user.Id, vpc, nil
}

func (nx *Nexodus) Stop() {
	nx.logger.Info("Stopping nexd")
	for _, proxy := range nx.proxies {
		proxy.Stop()
	}

	if nx.exitNode.exitNodeClientEnabled {
		nx.logger.Debugf("Stopping Exit Node Client")
		if err := nx.exitNodeClientTeardown(); err != nil {
			nx.logger.Errorf("failed to remove the exit node client configuration %v", err)
		}
	}

	if nx.exitNode.exitNodeOriginEnabled {
		nx.logger.Debugf("Stopping Exit Node Origin")
		if err := nx.exitNodeOriginTeardown(); err != nil {
			nx.logger.Errorf("failed to remove the exit node configuration %v", err)
		}
	}
}

// reconcileSecurityGroups will check the security group and update it if necessary.
func (nx *Nexodus) reconcileSecurityGroups(ctx context.Context) {
	if runtime.GOOS != Linux.String() && runtime.GOOS != Darwin.String() || nx.userspaceMode {
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
	securityGroups, httpResp, err := nx.securityGroupsInformer.Execute()
	if err != nil {
		// if the group ID returns a 404, clear the current rules
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			nx.securityGroup = nil
			if err := nx.processSecurityGroupRules(); err != nil {
				nx.logger.Error(err)
			}
			return
		}
		nx.logger.Errorf("Error retrieving the security groups: %v", err)
		return
	}

	responseSecGroup, found := securityGroups[existing.device.SecurityGroupId]
	if !found {
		nx.securityGroup = nil
		if err := nx.processSecurityGroupRules(); err != nil {
			nx.logger.Error(err)
		}
		nx.logger.Errorf("Error retrieving the security group")
		return
	}

	if nx.securityGroup != nil && reflect.DeepEqual(responseSecGroup, *nx.securityGroup) {
		// no changes to previously applied security group
		return
	}

	nx.logger.Debugf("Security Group change detected: %+v", responseSecGroup)
	oldSecGroup := nx.securityGroup
	nx.securityGroup = &responseSecGroup

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
		if !nx.deviceReconciled {
			nx.deviceReconciled = true
			nx.logger.Info("Nexodus agent has reconciled state with API server")
		}
		return
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.Temporary() {
		// Temporary dns resolution failure is normal, just debug log it
		nx.logger.Debugf("%v", err)
		return
	}

	nx.logger.Errorf("Failed to reconcile state with the nexodus API server: %v", err)
	nx.deviceReconciled = false

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

	informerCtx = nx.client.VPCApi.WatchEvents(informerCtx, nx.vpc.Id).NewSharedInformerContext()
	nx.securityGroupsInformer = nx.client.VPCApi.ListSecurityGroupsInVPC(informerCtx, nx.vpc.Id).Informer()
	nx.devicesInformer = nx.client.VPCApi.ListDevicesInVPC(informerCtx, nx.vpc.Id).Informer()

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
	// The most reliable method to passively check for an active wireguard session
	// is to check that the last handshake is within the REJECT_AFTER_TIME constant
	// defined in the wireguard paper (180 seconds).
	//
	// > After REJECT_AFTER_MESSAGES transport data messages or after the
	// > current secure session is REJECT_AFTER_TIME seconds old, whichever
	// > comes first, WireGuard will refuse to send or receive any more
	// > transport data messages using the current secure session, until a new
	// > secure session is created through the 1-RTT handshake.
	//
	// It is tempting to try to do something with the tx and rx counters availble
	// in the wireguard stats, but a past attempt helped determine that was not
	// reliable, as we can only count on keepalives going in one direction,
	// not both. For even more detail on this, check the git commit logs for this
	// file.
	//
	// For now, we will detect that peering is not working within about 3 minutes.

	if d.lastHandshakeTime.IsZero() {
		// We haven't seen a handshake yet, so this peer connection is not up.
		if d.peerHealthy {
			nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is unhealthy due to no handshake",
				d.device.Hostname, d.device.PublicKey,
				d.device.Endpoints[0].Address, d.device.Endpoints[1].Address)
		}
		return false
	}

	deadline := device.RejectAfterTime + (time.Second * 30)
	if time.Since(d.lastHandshakeTime) > deadline {
		// It has been too long since the last handshake, so this session has expired.
		if d.peerHealthy {
			nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is unhealthy due to lastHandshakeTime: %s > %s",
				d.device.Hostname, d.device.PublicKey,
				d.device.Endpoints[0].Address, d.device.Endpoints[1].Address,
				time.Since(d.lastHandshakeTime).String(), deadline.String())
		}
		return false
	}

	if !d.peerHealthy {
		nx.logger.Debugf("peer (hostname:%s pubkey:%s [%s %s]) is now healthy",
			d.device.Hostname, d.device.PublicKey,
			d.device.Endpoints[0].Address, d.device.Endpoints[1].Address)
	}

	return true
}

// assumes deviceCacheLock is held with a write-lock
func (nx *Nexodus) addToDeviceCache(p public.ModelsDevice) {
	d := deviceCacheEntry{
		device:      p,
		lastUpdated: time.Now(),
	}
	nx.peeringReset(&d)
	nx.deviceCache[p.PublicKey] = d
}

func (nx *Nexodus) reconcileDeviceCache() error {
	peerMap, resp, err := nx.devicesInformer.Execute()
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

	now := time.Now()

	nx.deviceCacheLock.Lock()
	defer nx.deviceCacheLock.Unlock()

	// Get our device cache up to date
	newLocalConfig := false
	for _, p := range peerMap {
		// Update the cache if the device is new or has changed
		existing, ok := nx.deviceCache[p.PublicKey]
		if !ok || nx.deviceUpdated(existing.device, p) {
			if p.PublicKey == nx.wireguardPubKey {
				newLocalConfig = true
			}
			nx.addToDeviceCache(p)
			existing = nx.deviceCache[p.PublicKey]
			delete(peerStats, p.PublicKey)
		}

		// Store the relay IP for easy reference later
		if p.Relay {
			nx.relayWgIP = p.AllowedIps[0]
		}

		// Keep track of peer connection stats for connection health tracking
		curStats, ok := peerStats[p.PublicKey]
		if !ok {
			if nx.wireguardPubKey != p.PublicKey && existing.peeringMethod != peeringMethodViaRelay {
				nx.logger.Debugf("peer (hostname:%s pubkey:%s) has no stats", p.Hostname, p.PublicKey)
			}
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
		existing.lastRefresh = now
		existing.endpoint = curStats.Endpoint
		existing.peerHealthy = nx.peerIsHealthy(existing)
		if existing.peerHealthy {
			existing.peerHealthyTime = now
		}
		nx.deviceCache[p.PublicKey] = existing
	}

	// Refresh wireguard peer configuration, getting any new peers or changes to existing peers
	updatePeers := nx.buildPeersConfig()
	if newLocalConfig || len(updatePeers) > 0 {
		for _, peer := range updatePeers {
			existing, ok := nx.deviceCache[peer.PublicKey]
			if !ok {
				continue
			}
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

// deviceUpdated() returns whether fields that impact peering configuration have changed
// between d1 and d2.
func (nx *Nexodus) deviceUpdated(d1, d2 public.ModelsDevice) bool {
	return !reflect.DeepEqual(d1.AllowedIps, d2.AllowedIps) ||
		!reflect.DeepEqual(d1.AdvertiseCidrs, d2.AdvertiseCidrs) ||
		!reflect.DeepEqual(d1.Endpoints, d2.Endpoints) ||
		d1.Relay != d2.Relay ||
		d1.SymmetricNat != d2.SymmetricNat
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
