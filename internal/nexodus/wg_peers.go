package nexodus

import (
	"net"
	"reflect"
	"runtime"
	"strings"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

type wgPeerMethod struct {
	// name of the peering method
	name string
	// determine if this peering method is available for the given device
	checkPrereqs func(nx *Nexodus, device public.ModelsDevice, reflexiveIP4 string) bool
	// buildPeerConfig builds the peer configuration for a given peering method
	buildPeerConfig func(nx *Nexodus, device public.ModelsDevice, relayAllowedIP []string, localIP, peerPort, reflexiveIP4 string) wgPeerConfig
}

// list of peering methods, in order of preference
var wgPeerMethods = []wgPeerMethod{
	{
		// This node is a relay node and we have the same reflexive address as the peer
		name: "relay-node-self-direct-local",
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, reflexiveIP4 string) bool {
			return nx.relay && nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalPeerForRelayNode,
	},
	{
		// This node is a relay node
		name: "relay-node-self",
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, _ string) bool {
			return nx.relay
		},
		buildPeerConfig: buildPeerForRelayNode,
	},
	{
		// The peer is a relay node and we have the same reflexive address as the peer
		name: "relay-node-peer-direct-local",
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, reflexiveIP4 string) bool {
			return device.Relay && nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalRelayPeer,
	},
	{
		// The peer is a relay node
		name: "relay-node-peer",
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, _ string) bool {
			return device.Relay
		},
		buildPeerConfig: buildRelayPeer,
	},
	{
		// We are behind the same reflexive address as the peer, try direct, local peering
		name: "direct-local",
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, reflexiveIP4 string) bool {
			return nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalPeer,
	},
	{
		// If neither side is behind symmetric NAT, we can try peering with its reflexive address.
		// This is the address+port opened up by the peer using STUN.
		name: "reflexive",
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, _ string) bool {
			return !device.SymmetricNat && !nx.symmetricNat
		},
		buildPeerConfig: buildReflexivePeer,
	},
	// If no peer-specific configuration was generated, connectivity will be handled by a relay node
	// if one is available. That configuration is generated when determining the peering method for
	// the relay node itself.
}

// buildPeersConfig builds the peer configuration based off peer cache
// and peer listings from the controller. assumes deviceCacheLock is held.
func (nx *Nexodus) buildPeersConfig() map[string]public.ModelsDevice {
	if nx.wgConfig.Peers == nil {
		nx.wgConfig.Peers = map[string]wgPeerConfig{}
	}

	updatedPeers := map[string]public.ModelsDevice{}

	_, nx.wireguardPubKeyInConfig = nx.deviceCache[nx.wireguardPubKey]

	relayAllowedIP := []string{
		nx.org.Cidr,
		nx.org.CidrV6,
	}

	nx.buildLocalConfig()

	for _, d := range nx.deviceCache {
		// skip ourselves
		if d.device.PublicKey == nx.wireguardPubKey {
			continue
		}

		localIP, reflexiveIP4 := nx.extractLocalAndReflexiveIP(d.device)
		peerPort := nx.extractPeerPort(localIP)

		for _, method := range wgPeerMethods {
			if !method.checkPrereqs(nx, d.device, reflexiveIP4) {
				continue
			}
			peer := method.buildPeerConfig(nx, d.device, relayAllowedIP, localIP, peerPort, reflexiveIP4)
			if nx.peerConfigUpdated(d.device, peer) {
				updatedPeers[d.device.PublicKey] = d.device
				nx.wgConfig.Peers[d.device.PublicKey] = peer
				nx.logPeerInfo(d.device, peer.Endpoint, method.name)
			}
			break
		}
	}

	return updatedPeers
}

func (nx *Nexodus) peerConfigUpdated(device public.ModelsDevice, peer wgPeerConfig) bool {
	if _, ok := nx.wgConfig.Peers[device.PublicKey]; !ok {
		return true
	}

	if nx.wgConfig.Peers[device.PublicKey].Endpoint != peer.Endpoint {
		return true
	}

	if !reflect.DeepEqual(nx.wgConfig.Peers[device.PublicKey].AllowedIPs, peer.AllowedIPs) {
		return true
	}

	return false
}

// extractLocalAndReflexiveIP retrieve the local and reflexive endpoint addresses
func (nx *Nexodus) extractLocalAndReflexiveIP(device public.ModelsDevice) (string, string) {
	localIP := ""
	reflexiveIP4 := ""
	for _, endpoint := range device.Endpoints {
		if endpoint.Source == "local" {
			localIP = endpoint.Address
		} else {
			reflexiveIP4 = endpoint.Address
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

func buildDirectLocalPeerForRelayNode(nx *Nexodus, device public.ModelsDevice, _ []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            localIP,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildPeerForRelayNode build a config for all peers if this node is the organization's relay node.
// The peer for a relay node is currently left blank and assumed to be exposed to all peers, we still build its peer config for flexibility.
func buildPeerForRelayNode(nx *Nexodus, device public.ModelsDevice, _ []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

func buildDirectLocalRelayPeer(nx *Nexodus, device public.ModelsDevice, relayAllowedIP []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            localIP,
		AllowedIPs:          relayAllowedIP,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildRelayPeer Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
// This is the only peer a symmetric NAT node will get unless it also has a direct peering
func buildRelayPeer(nx *Nexodus, device public.ModelsDevice, relayAllowedIP []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          relayAllowedIP,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildDirectLocalPeer If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
func buildDirectLocalPeer(nx *Nexodus, device public.ModelsDevice, _ []string, localIP, peerPort, _ string) wgPeerConfig {
	directLocalPeerEndpointSocket := net.JoinHostPort(device.EndpointLocalAddressIp4, peerPort)
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            directLocalPeerEndpointSocket,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildReflexive Peer the bulk of the peers will be added here except for local address peers or
// symmetric NAT peers or if this device is itself a symmetric nat node, that require relaying.
func buildReflexivePeer(nx *Nexodus, device public.ModelsDevice, _ []string, _, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

func (nx *Nexodus) logPeerInfo(device public.ModelsDevice, endpointIP, method string) {
	nx.logger.Debugf("Peer configuration - Method [ %s ] Peer AllowedIps [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ]",
		method,
		strings.Join(device.AllowedIps, ", "),
		endpointIP,
		device.PublicKey)
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
	if nx.TunnelIP != d.device.TunnelIp {
		nx.logger.Infof("New local Wireguard interface addresses assigned IPv4 [ %s ] IPv6 [ %s ]", d.device.TunnelIp, d.device.TunnelIpV6)
		if runtime.GOOS == Linux.String() && linkExists(nx.tunnelIface) {
			if err := delLink(nx.tunnelIface); err != nil {
				nx.logger.Infof("Failed to delete %s: %v", nx.tunnelIface, err)
			}
		}
	}
	nx.TunnelIP = d.device.TunnelIp
	nx.TunnelIpV6 = d.device.TunnelIpV6
	localInterface = wgLocalConfig{
		nx.wireguardPvtKey,
		nx.listenPort,
	}
	// set the node unique local interface configuration
	nx.wgConfig.Interface = localInterface
}
