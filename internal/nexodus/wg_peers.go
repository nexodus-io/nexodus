package nexodus

import (
	"net"
	"runtime"
	"strings"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

const (
	defaultOrganizationPrefixIPv4 = "100.100.0.0/16"
	defaultOrganizationPrefixIPv6 = "0200::/64"
)

// buildPeersConfig builds the peer configuration based off peer cache and peer listings from the controller
func (ax *Nexodus) buildPeersConfig() {
	peers := ax.buildPeersAndRelay()
	ax.wgConfig.Peers = peers
}

// buildPeersAndRelay constructs the peer configuration returning it as []wgPeerConfig.
// This also call the method for building the local interface configuration wgLocalConfig.
func (ax *Nexodus) buildPeersAndRelay() []wgPeerConfig {
	var peers []wgPeerConfig

	_, ax.wireguardPubKeyInConfig = ax.deviceCache[ax.wireguardPubKey]
	for _, d := range ax.deviceCache {
		if d.device.Relay {
			ax.relayWgIP = d.device.AllowedIps[0]
			break
		}
	}

	relayAllowedIP := []string{
		defaultOrganizationPrefixIPv4,
		defaultOrganizationPrefixIPv6,
	}

	ax.buildLocalConfig()

	for _, d := range ax.deviceCache {
		// skip ourselves
		if d.device.PublicKey == ax.wireguardPubKey {
			continue
		}

		localIP, reflexiveIP4 := ax.extractLocalAndReflexiveIP(d.device)
		peerPort := ax.extractPeerPort(localIP)

		// We are a relay node. This block will get hit for every peer.
		if ax.relay {
			peer := ax.buildPeerForRelayNode(d.device, localIP, reflexiveIP4)
			peers = append(peers, peer)
			ax.logPeerInfo(d.device, reflexiveIP4)
			continue
		}

		// The peer is a relay node
		if d.device.Relay {
			peerRelay := ax.buildRelayPeer(d.device, relayAllowedIP, localIP, reflexiveIP4)
			peers = append(peers, peerRelay)
			ax.logPeerInfo(d.device, peerRelay.Endpoint)
			continue
		}

		// We are behind the same reflexive address as the peer, try local peering first
		if ax.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4) {
			peer := ax.buildDirectLocalPeer(d.device, localIP, peerPort)
			peers = append(peers, peer)
			ax.logPeerInfo(d.device, localIP)
			continue
		}

		// If we are behind symmetric NAT, we have no further options
		if ax.symmetricNat {
			continue
		}

		// If the peer is not behind symmetric NAT, we can try peering with its reflexive address
		if !d.device.SymmetricNat {
			peer := ax.buildDefaultPeer(d.device, reflexiveIP4)
			peers = append(peers, peer)
			ax.logPeerInfo(d.device, reflexiveIP4)
		}
	}

	return peers
}

// extractLocalAndReflexiveIP retrieve the local and reflexive endpoint addresses
func (ax *Nexodus) extractLocalAndReflexiveIP(device public.ModelsDevice) (string, string) {
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

func (ax *Nexodus) extractPeerPort(localIP string) string {
	_, port, err := net.SplitHostPort(localIP)
	if err != nil {
		ax.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v", err)
		return ""
	}
	return port
}

// buildRelayPeer Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
// This is the only peer a symmetric NAT node will get unless it also has a direct peering
func (ax *Nexodus) buildRelayPeer(device public.ModelsDevice, relayAllowedIP []string, localIP, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	if ax.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4) {
		return wgPeerConfig{
			PublicKey:           device.PublicKey,
			Endpoint:            localIP,
			AllowedIPs:          relayAllowedIP,
			PersistentKeepAlive: persistentKeepalive,
		}
	}
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          relayAllowedIP,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildPeerForRelayNode build a config for all peers if this node is the organization's relay node. Also check for direct peering.
// The peer for a relay node is currently left blank and assumed to be exposed to all peers, we still build its peer config for flexibility.
func (ax *Nexodus) buildPeerForRelayNode(device public.ModelsDevice, localIP, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	if ax.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4) {
		return wgPeerConfig{
			PublicKey:           device.PublicKey,
			Endpoint:            localIP,
			AllowedIPs:          device.AllowedIps,
			PersistentKeepAlive: persistentKeepalive,
		}
	}
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildDirectLocalPeer If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
func (ax *Nexodus) buildDirectLocalPeer(device public.ModelsDevice, localIP, peerPort string) wgPeerConfig {
	directLocalPeerEndpointSocket := net.JoinHostPort(device.EndpointLocalAddressIp4, peerPort)
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            directLocalPeerEndpointSocket,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildDefaultPeer the bulk of the peers will be added here except for local address peers or
// symmetric NAT peers or if this device is itself a symmetric nat node, that require relaying.
func (ax *Nexodus) buildDefaultPeer(device public.ModelsDevice, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.ChildPrefix...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

func (ax *Nexodus) logPeerInfo(device public.ModelsDevice, endpointIP string) {
	ax.logger.Debugf("Peer Configuration - Peer AllowedIps [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ]",
		strings.Join(device.AllowedIps, ", "),
		endpointIP,
		device.PublicKey)
}

// buildLocalConfig builds the configuration for the local interface
func (ax *Nexodus) buildLocalConfig() {
	var localInterface wgLocalConfig
	var d deviceCacheEntry
	var ok bool

	if d, ok = ax.deviceCache[ax.wireguardPubKey]; !ok {
		return
	}

	// if the local node address changed replace it on wg0
	if ax.TunnelIP != d.device.TunnelIp {
		ax.logger.Infof("New local Wireguard interface addresses assigned IPv4 [ %s ] IPv6 [ %s ]", d.device.TunnelIp, d.device.TunnelIpV6)
		if runtime.GOOS == Linux.String() && linkExists(ax.tunnelIface) {
			if err := delLink(ax.tunnelIface); err != nil {
				ax.logger.Infof("Failed to delete %s: %v", ax.tunnelIface, err)
			}
		}
	}
	ax.TunnelIP = d.device.TunnelIp
	ax.TunnelIpV6 = d.device.TunnelIpV6
	localInterface = wgLocalConfig{
		ax.wireguardPvtKey,
		ax.listenPort,
	}
	ax.logger.Debugf("Local Node Configuration - Wireguard IPv4 [ %s ] IPv6 [ %s ]", ax.TunnelIP, ax.TunnelIpV6)
	// set the node unique local interface configuration
	ax.wgConfig.Interface = localInterface
}
