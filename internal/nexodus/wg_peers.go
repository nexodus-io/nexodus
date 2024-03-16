package nexodus

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/client"
	"net"
	"net/netip"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	// How long to wait for successful peering after choosing a new peering method
	peeringTimeout = time.Second * 30
	// How long to wait for peering to successfully restore itself after seeing
	// successful peering using a given method, but it goes down.
	peeringRestoreTimeout             = time.Second * 180
	peeringMethodRelaySelfDirectLocal = "relay-node-self-direct-local"
	peeringMethodRelaySelf            = "relay-node-self"
	peeringMethodRelayPeerDirectLocal = "relay-node-peer-direct-local"
	peeringMethodRelayPeer            = "relay-node-peer"
	peeringMethodDirectLocal          = "direct-local"
	peeringMethodReflexive            = "reflexive"
	peeringMethodViaRelay             = "via-relay"
	peeringMethodViaDerpRelay         = "via-derp-relay"
	peeringMethodNone                 = "none"
)

type wgPeerMethod struct {
	// name of the peering method
	name string
	// determine if this peering method is available for the given device
	checkPrereqs func(nx *Nexodus, device client.ModelsDevice, reflexiveIP4 string, healthyRelay bool, wgRelayAvailable bool) bool
	// buildPeerConfig builds the peer configuration for a given peering method
	buildPeerConfig func(nx *Nexodus, device client.ModelsDevice, relayAllowedIP []string, localIP, peerPort, reflexiveIP4 string) wgPeerConfig
}

// list of peering methods, in order of preference
var wgPeerMethods = []wgPeerMethod{
	{
		// This node is a relay node and we have the same reflexive address as the peer
		name: peeringMethodRelaySelfDirectLocal,
		checkPrereqs: func(nx *Nexodus, device client.ModelsDevice, reflexiveIP4 string, healthyRelay bool, _ bool) bool {
			return nx.relay && nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalPeerForRelayNode,
	},
	{
		// This node is a relay node
		name: peeringMethodRelaySelf,
		checkPrereqs: func(nx *Nexodus, device client.ModelsDevice, _ string, healthyRelay bool, _ bool) bool {
			return nx.relay
		},
		buildPeerConfig: buildPeerForRelayNode,
	},
	{
		// The peer is a relay node and we have the same reflexive address as the peer
		name: peeringMethodRelayPeerDirectLocal,
		checkPrereqs: func(nx *Nexodus, device client.ModelsDevice, reflexiveIP4 string, healthyRelay bool, _ bool) bool {
			return !nx.relay && device.GetRelay() && nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalRelayPeer,
	},
	{
		// The peer is a relay node
		name: peeringMethodRelayPeer,
		checkPrereqs: func(nx *Nexodus, device client.ModelsDevice, _ string, healthyRelay bool, _ bool) bool {
			return !nx.relay && device.GetRelay()
		},
		buildPeerConfig: buildRelayPeer,
	},
	{
		// We are behind the same reflexive address as the peer, try direct, local peering
		name: peeringMethodDirectLocal,
		checkPrereqs: func(nx *Nexodus, device client.ModelsDevice, reflexiveIP4 string, healthyRelay bool, _ bool) bool {
			return !nx.relay && !nx.relayOnly && !device.GetRelay() && nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalPeer,
	},
	{
		// If neither side is behind symmetric NAT, we can try peering with its reflexive address.
		// This is the address+port opened up by the peer using STUN.
		name: peeringMethodReflexive,
		checkPrereqs: func(nx *Nexodus, device client.ModelsDevice, _ string, healthyRelay bool, _ bool) bool {
			return !nx.relay && !nx.relayOnly && !device.GetRelay() && !device.GetSymmetricNat() && !nx.symmetricNat
		},
		buildPeerConfig: buildReflexivePeer,
	},
	{
		// Try connecting to the peer via a derp relay, in case the legacy relay is not available
		// and none of the peering methods above worked
		name: peeringMethodViaDerpRelay,
		checkPrereqs: func(nx *Nexodus, device client.ModelsDevice, reflexiveIP4 string, healthyRelay bool, wgRelayAvailable bool) bool {
			return !nx.relay && !device.GetRelay() && // don't use if either peer is a relay.
				!wgRelayAvailable && // don't use if a wg relay is available...
				(nx.symmetricNat || device.GetSymmetricNat()) // use if one of the peers on a symmetric nat.
		},
		buildPeerConfig: buildPeerViaDerpRelay,
	},
	{
		// Last chance, try connecting to the peer via a wireguard relay
		name: peeringMethodViaRelay,
		checkPrereqs: func(nx *Nexodus, device client.ModelsDevice, _ string, healthyRelay bool, wgRelayAvailable bool) bool {
			return !nx.relay && !device.GetRelay() && healthyRelay && wgRelayAvailable
		},
		buildPeerConfig: func(nx *Nexodus, device client.ModelsDevice, _ []string, _, _, _ string) wgPeerConfig {
			return wgPeerConfig{
				AllowedIPsForRelay: device.AdvertiseCidrs,
			}
		},
	},
}

func (nx *Nexodus) peeringReset(d *deviceCacheEntry) {
	nx.logger.Debugf("Resetting peer configuration - Peer AllowedIps [ %s ] Peer Public Key [ %s ]",
		strings.Join(d.device.AllowedIps, ", "), d.device.GetPublicKey())

	d.peeringMethod = peeringMethodNone
	// By setting the peering method index to -1, we will consider all other
	// methods that may be available.
	d.peeringMethodIndex = -1

	// All the stats are now invalid
	d.peeringTime = time.Time{}
	d.peerHealthy = false
	d.peerHealthyTime = time.Time{}
	d.lastTxBytes = 0
	d.lastRxBytes = 0
	d.lastHandshakeTime = time.Time{}
	d.lastHandshake = ""
	d.lastRefresh = time.Time{}
}

// shouldResetPeering() determines if we should reset peering to start over at the
// beginning of the peering list.
func (nx *Nexodus) shouldResetPeering(d *deviceCacheEntry, reflexiveIP4 string, healthyRelay bool, wgRelayAvailable bool) bool {
	if d.peeringMethodIndex == -1 {
		// Already in a reset state
		return false
	}

	if d.peeringMethodIndex == len(wgPeerMethods)-1 {
		// We've reached the end of the peering method list, time to reset
		return true
	}

	// If not at the end, check to see if the prerequisites pass for any of the following methods
	for i := d.peeringMethodIndex + 1; i < len(wgPeerMethods); i++ {
		if wgPeerMethods[i].checkPrereqs(nx, d.device, reflexiveIP4, healthyRelay, wgRelayAvailable) {
			// Prequisites pass for this method, so don't reset
			return false
		}
	}

	// There are no methods remaining that have passed the prerequisites, so reset
	return true
}

func (nx *Nexodus) rebuildPeerConfig(d *deviceCacheEntry, healthyRelay bool, wgRelayAvailable bool) (wgPeerConfig, string, int) {
	localIP, reflexiveIP4 := nx.extractLocalAndReflexiveIP(d.device)
	peerPort := nx.extractPeerPort(localIP)
	relayAllowedIP := []string{
		nx.vpc.GetIpv4Cidr(),
		nx.vpc.GetIpv6Cidr(),
	}

	tryNextMethod := nx.peeringFailed(*d, healthyRelay)
	if tryNextMethod {
		nx.logger.Debugf("Peering with peer [ %s ] using method [ %s ] has failed, trying next method", d.device.GetPublicKey(), d.peeringMethod)
		if nx.shouldResetPeering(d, reflexiveIP4, healthyRelay, wgRelayAvailable) {
			// We failed to connect via a relay, which is the last resort, so start over at the beginning
			nx.peeringReset(d)
			tryNextMethod = false
		}
	}

	peer := nx.wgConfig.Peers[d.device.GetPublicKey()]
	chosenMethod := d.peeringMethod
	chosenMethodIndex := d.peeringMethodIndex
	for i, method := range wgPeerMethods {
		if i < d.peeringMethodIndex {
			// A peering method was previously chosen and we haven't reached it yet
			continue
		}
		if i == d.peeringMethodIndex && tryNextMethod {
			// A peering method was previously chosen and it failed
			continue
		}
		if !method.checkPrereqs(nx, d.device, reflexiveIP4, healthyRelay, wgRelayAvailable) {
			// This peering method is not a candidate for this peer, the prereqs failed
			continue
		}
		if method.name == peeringMethodViaRelay && d.peeringMethod == peeringMethodViaRelay {
			// We are already set up to use a relay for this peer
			break
		}
		peer = method.buildPeerConfig(nx, d.device, relayAllowedIP, localIP, peerPort, reflexiveIP4)
		chosenMethod = method.name
		chosenMethodIndex = i
		break
	}

	return peer, chosenMethod, chosenMethodIndex
}

// buildPeersConfig builds the peer configuration based off peer cache
// and peer listings from the controller. assumes deviceCacheLock is held.
// Returns a map of peer public keys to devices that have had their wireguard
// configuration updated.
func (nx *Nexodus) buildPeersConfig() map[string]client.ModelsDevice {
	if nx.wgConfig.Peers == nil {
		nx.wgConfig.Peers = map[string]wgPeerConfig{}
	}

	updatedPeers := map[string]client.ModelsDevice{}
	allowedIPsForRelay := []string{}

	_, nx.wireguardPubKeyInConfig = nx.deviceCache[nx.wireguardPubKey]

	nx.buildLocalConfig()

	// do we have a healthy relay available?
	healthyRelay := false
	relayAvailable := false
	isDerpRelay := false
	var relayDevice deviceCacheEntry
	for _, d := range nx.deviceCache {
		if d.device.GetRelay() {
			relayAvailable = true
			relayDevice = d
			isDerpRelay = nx.derpRelay(d)
			if d.peerHealthy {
				healthyRelay = true
				break
			}
		}
	}

	// If on-boarded relay is available and it's derp relay, set custom derp map
	// If there is no on-boarded relay, set default derp map
	// if there is on-boarded relay, but it's wireguard relay, skip derp map setting
	// This is to ensure that if wireguard relay is on-boarded, legacy control flow
	// should work the same as before.
	if relayAvailable && isDerpRelay {
		var derpAddr string
		for _, addr := range relayDevice.device.Endpoints {
			if addr.GetSource() == "stun:" {
				derpAddr = addr.GetAddress()
			}
		}

		if derpAddr == "" {
			nx.logger.Warnf("no STUN endpoint found for relay device %v", relayDevice.metadata.GetDeviceId())
		}
		hostname, err := nx.getDerpRelayHostname(relayDevice)
		if err != nil {
			nx.logger.Warnf("failed to get hostname for relay device %v: %v", relayDevice.metadata.GetDeviceId(), err)
		}

		if hostname != "" && derpAddr != "" {
			if nx.nexRelay.myDerp == DefaultDerpRegionID {
				nx.logger.Debugf("User on-boarded derp relay is available, switching to on-boarded relay from public relay.")
				nx.nexRelay.closeDerpLocked(nx.nexRelay.myDerp, "switching to custom DERP map")
			}
			if nx.nexRelay.myDerp == 0 {
				nx.logger.Debugf("User on-boarded derp relay is available, use it for region: [ %d] ", CustomDerpRegionID)
			}

			if nx.nexRelay.myDerp != CustomDerpRegionID {
				nx.nexRelay.SetCustomDERPMap(derpAddr, hostname)
				cmManual := nx.certModeManual(relayDevice)

				if cmManual {
					// If the cert mode is manual, we need to set the local DNS entry
					// to point it to the provided hostname
					ip, _, err := net.SplitHostPort(derpAddr)
					if err != nil {
						nx.logger.Errorf("Failed to parse stun address %s : %v", derpAddr, err)
					}

					addr, err := netip.ParseAddr(ip)
					if err != nil {
						nx.logger.Errorf("Failed to parse ip address %s : %v", ip, err)
					}

					nx.nexRelay.inMemResolver.Set(hostname, []netip.Addr{addr})
				}
			}

		}
	} else if !relayAvailable {
		if nx.nexRelay.myDerp == CustomDerpRegionID {
			nx.nexRelay.logger.Debugf("User on-boarded derp relay is gone, lets default to hosted relay %v", nx.nexRelay.derpMap.Regions[DefaultDerpRegionID])
			nx.nexRelay.closeDerpLocked(nx.nexRelay.myDerp, "switching to default DERP map")

			// Clean up the local DNS entry set for onboarded derp relay
			hostname, err := nx.nexRelay.getDerpRelayHostname(nx.nexRelay.myDerp)
			if err != nil {
				nx.nexRelay.logger.Warnf("failed to get hostname for derp relay device %s: %v", relayDevice.metadata.GetDeviceId(), err)
			} else {
				nx.nexRelay.inMemResolver.Delete(hostname)
			}
		}
		nx.nexRelay.SetDefaultDERPMap()

	}

	now := time.Now()
	wgRelayAvailable := relayAvailable && !isDerpRelay
	for _, dIter := range nx.deviceCache {
		d := dIter
		// skip ourselves
		if d.device.GetPublicKey() == nx.wireguardPubKey {
			continue
		}

		peerConfig, chosenMethod, chosenMethodIndex := nx.rebuildPeerConfig(&d, healthyRelay, wgRelayAvailable)
		if len(peerConfig.AllowedIPsForRelay) > 0 {
			allowedIPsForRelay = append(allowedIPsForRelay, peerConfig.AllowedIPsForRelay...)
		}

		if !nx.peerConfigUpdated(d.device, peerConfig) {
			// The resulting peer configuration hasn't changed.
			continue
		}

		updatedPeers[d.device.GetPublicKey()] = d.device
		if chosenMethod == peeringMethodViaRelay {
			// When switching to a relay, we have no configuration to connect directly to the peer
			if _, ok := nx.wgConfig.Peers[d.device.GetPublicKey()]; ok {
				delete(nx.wgConfig.Peers, d.device.GetPublicKey())
				_ = nx.peerCleanup(d.device)
			}
		} else {
			nx.wgConfig.Peers[d.device.GetPublicKey()] = peerConfig
		}
		d.peeringMethodIndex = chosenMethodIndex
		d.peeringMethod = chosenMethod
		d.peeringTime = now
		nx.deviceCache[d.device.GetPublicKey()] = d
		nx.logPeerInfo(d.device, peerConfig.Endpoint, chosenMethod)
	}

	if healthyRelay && len(allowedIPsForRelay) > 0 {
		// Add child prefix CIDRs to the relay for peers that we can only reach via the relay
		relayConfig := nx.wgConfig.Peers[relayDevice.device.GetPublicKey()]
		relayConfig.AllowedIPs = append([]string{nx.vpc.GetIpv4Cidr(), nx.vpc.GetIpv4Cidr()}, allowedIPsForRelay...)
		nx.wgConfig.Peers[relayDevice.device.GetPublicKey()] = relayConfig
	}

	return updatedPeers
}

func (nx *Nexodus) peeringFailed(d deviceCacheEntry, healthyRelay bool) bool {
	if d.peerHealthy {
		return false
	}

	if d.peeringMethodIndex == len(wgPeerMethods)-1 {
		return !healthyRelay
	}

	if d.peeringTime.IsZero() {
		// haven't even tried yet ...
		return false
	}

	if d.peerHealthyTime.IsZero() && time.Since(d.peeringTime) < peeringTimeout {
		// Peering has never been successful since choosing this method,
		// so time out quicker than if it had worked and we're waiting for it to come back up.
		return false
	}

	if !d.peerHealthyTime.IsZero() && time.Since(d.peerHealthyTime) < peeringRestoreTimeout {
		// Peering worked, but went down, so give it a few minutes to come back up.
		return false
	}

	return true
}

func (nx *Nexodus) peerConfigUpdated(device client.ModelsDevice, peer wgPeerConfig) bool {
	if _, ok := nx.wgConfig.Peers[device.GetPublicKey()]; !ok {
		return true
	}

	if nx.wgConfig.Peers[device.GetPublicKey()].Endpoint != peer.Endpoint {
		return true
	}

	if !reflect.DeepEqual(nx.wgConfig.Peers[device.GetPublicKey()].AllowedIPs, peer.AllowedIPs) {
		return true
	}

	return false
}

// extractLocalAndReflexiveIP retrieve the local and reflexive endpoint addresses
func (nx *Nexodus) extractLocalAndReflexiveIP(device client.ModelsDevice) (string, string) {
	localIP := ""
	reflexiveIP4 := ""
	for _, endpoint := range device.Endpoints {
		if endpoint.GetSource() == "local" {
			localIP = endpoint.GetAddress()
		} else {
			reflexiveIP4 = endpoint.GetAddress()
		}
	}
	return localIP, reflexiveIP4
}

func (nx *Nexodus) extractPeerPort(localIP string) string {
	_, port, err := net.SplitHostPort(localIP)
	if err != nil {
		nx.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v", err)
		return ""
	}
	return port
}

func buildDirectLocalPeerForRelayNode(nx *Nexodus, device client.ModelsDevice, _ []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.GetPublicKey(),
		Endpoint:            localIP,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildPeerForRelayNode build a config for all peers if this node is the organization's relay node.
// The peer for a relay node is currently left blank and assumed to be exposed to all peers, we still build its peer config for flexibility.
func buildPeerForRelayNode(nx *Nexodus, device client.ModelsDevice, _ []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.GetPublicKey(),
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

func buildDirectLocalRelayPeer(nx *Nexodus, device client.ModelsDevice, relayAllowedIP []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.GetPublicKey(),
		Endpoint:            localIP,
		AllowedIPs:          relayAllowedIP,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildRelayPeer Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
// This is the only peer a symmetric NAT node will get unless it also has a direct peering
func buildRelayPeer(nx *Nexodus, device client.ModelsDevice, relayAllowedIP []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.GetPublicKey(),
		Endpoint:            reflexiveIP4,
		AllowedIPs:          relayAllowedIP,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildDirectLocalPeer If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
func buildDirectLocalPeer(nx *Nexodus, device client.ModelsDevice, _ []string, localIP, _, _ string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.GetPublicKey(),
		Endpoint:            localIP,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildReflexive Peer the bulk of the peers will be added here except for local address peers or
// symmetric NAT peers or if this device is itself a symmetric nat node, that require relaying.
func buildReflexivePeer(nx *Nexodus, device client.ModelsDevice, _ []string, _, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.GetPublicKey(),
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildPeerViaDerpRelay Peer and this node, both are behind symmetric NAT, so the only option is to peer them via the derp relay
func buildPeerViaDerpRelay(nx *Nexodus, device client.ModelsDevice, _ []string, _, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	ip, err := nx.nexRelay.derpIpMapping.GetLocalIPMappingForPeer(device.GetPublicKey())
	if err != nil {
		nx.logger.Errorf("Failed to get next available ip address from the pool: %v", err)
		return wgPeerConfig{}
	}
	err = nx.configureLoopback(ip)
	if err != nil {
		nx.logger.Errorf("Failed to configure loopback interface: %v", err)
		return wgPeerConfig{}
	}
	ip = net.JoinHostPort(ip, strconv.Itoa(nx.nexRelay.myDerp))
	return wgPeerConfig{
		PublicKey:           device.GetPublicKey(),
		Endpoint:            ip,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

func (nx *Nexodus) logPeerInfo(device client.ModelsDevice, endpointIP, method string) {
	nx.logger.Debugf("Peer configuration - Method [ %s ] Peer AllowedIps [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ]",
		method,
		strings.Join(device.AllowedIps, ", "),
		endpointIP,
		device.GetPublicKey())
}

// buildLocalConfig builds the configuration for the local interface
func (nx *Nexodus) buildLocalConfig() {
	var localInterface wgLocalConfig
	var d deviceCacheEntry
	var ok bool

	if d, ok = nx.deviceCache[nx.wireguardPubKey]; !ok {
		return
	}

	// if the local node address changed replace it on wg0
	if nx.TunnelIP != d.device.Ipv4TunnelIps[0].GetAddress() {
		nx.logger.Infof("New local Wireguard interface addresses assigned IPv4 [ %s ] IPv6 [ %s ]",
			d.device.Ipv4TunnelIps[0].GetAddress(), d.device.Ipv6TunnelIps[0].GetAddress())
		if runtime.GOOS == Linux.String() && linkExists(nx.tunnelIface) {
			if err := delLink(nx.tunnelIface); err != nil {
				nx.logger.Infof("Failed to delete %s: %v", nx.tunnelIface, err)
			}
		}
	}
	nx.TunnelIP = d.device.Ipv4TunnelIps[0].GetAddress()
	nx.TunnelIpV6 = d.device.Ipv6TunnelIps[0].GetAddress()
	localInterface = wgLocalConfig{
		nx.wireguardPvtKey,
		nx.listenPort,
	}
	// set the node unique local interface configuration
	nx.wgConfig.Interface = localInterface
}

func (nx *Nexodus) derpRelay(d deviceCacheEntry) bool {
	rtype, ok := d.metadata.Value["type"]
	if !ok {
		return false
	} else if rtype == "derp" {
		return true
	}
	return false
}

func (nx *Nexodus) getDerpRelayHostname(d deviceCacheEntry) (string, error) {
	hostname, ok := d.metadata.Value["hostname"]
	if !ok {
		return "", fmt.Errorf("derp hostname metadata not found")
	}
	return hostname.(string), nil
}

func (nx *Nexodus) certModeManual(d deviceCacheEntry) bool {
	rtype, ok := d.metadata.Value["certmodemanual"]
	if !ok {
		return false
	}
	return rtype.(bool)
}
