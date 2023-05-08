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

	for _, device := range ax.deviceCache {
		if device.PublicKey == ax.wireguardPubKey {
			ax.wireguardPubKeyInConfig = true
		}
		if device.Relay {
			ax.relayWgIP = device.AllowedIps[0]
		}
	}

	relayAllowedIP := []string{
		defaultOrganizationPrefixIPv4,
		defaultOrganizationPrefixIPv6,
	}

	ax.buildLocalConfig()

	for _, device := range ax.deviceCache {
		localIP, reflexiveIP4 := ax.extractLocalAndReflexiveIP(device)
		peerPort := ax.extractPeerPort(localIP)
		if device.PublicKey == ax.wireguardPubKey {
			continue
		}

		if !ax.relay && device.Relay {
			peerRelay := ax.buildRelayPeer(device, relayAllowedIP, localIP, reflexiveIP4)
			peers = append(peers, peerRelay)
			continue
		}

		if ax.relay {
			peer := ax.buildPeerForRelayNode(device, localIP, reflexiveIP4)
			peers = append(peers, peer)
			ax.logPeerInfo(device, reflexiveIP4)
			continue
		}

		if ax.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4) && !device.Relay {
			peer := ax.buildDirectLocalPeer(device, localIP, peerPort)
			peers = append(peers, peer)
			ax.logPeerInfo(device, localIP)

		} else if !ax.symmetricNat && !device.SymmetricNat && !device.Relay {
			peer := ax.buildDefaultPeer(device, reflexiveIP4)
			peers = append(peers, peer)
			ax.logPeerInfo(device, reflexiveIP4)
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

	for _, value := range ax.deviceCache {
		// build the local interface configuration if this node is an Organization router
		if value.PublicKey == ax.wireguardPubKey {
			// if the local node address changed replace it on wg0
			if ax.TunnelIP != value.TunnelIp {
				ax.logger.Infof("New local Wireguard interface addresses assigned IPv4 [ %s ] IPv6 [ %s ]", value.TunnelIp, value.TunnelIpV6)
				if runtime.GOOS == Linux.String() && linkExists(ax.tunnelIface) {
					if err := delLink(ax.tunnelIface); err != nil {
						ax.logger.Infof("Failed to delete %s: %v", ax.tunnelIface, err)
					}
				}
			}
			ax.TunnelIP = value.TunnelIp
			ax.TunnelIpV6 = value.TunnelIpV6
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				ax.listenPort,
			}
			ax.logger.Debugf("Local Node Configuration - Wireguard IPv4 [ %s ] IPv6 [ %s ]", ax.TunnelIP, ax.TunnelIpV6)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
	}
}
