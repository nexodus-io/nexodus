package nexodus

import (
	"net"
	"runtime"
)

const (
	defaultOrganizationPrefixIPv4 = "100.100.0.0/16"
	defaultOrganizationPrefixIPv6 = "0200::/64"
)

// buildPeersConfig builds the peer configuration based off peer cache and peer listings from the controller
func (ax *Nexodus) buildPeersConfig() {

	var peers []wgPeerConfig
	var relayIP string
	for _, device := range ax.deviceCache {
		if device.PublicKey == ax.wireguardPubKey {
			ax.wireguardPubKeyInConfig = true
		}
		if device.Relay {
			relayIP = device.AllowedIps[0]
		}
	}
	relayAllowedIP := []string{
		defaultOrganizationPrefixIPv4,
		defaultOrganizationPrefixIPv6,
	}

	// Build the local interface configuration
	ax.buildLocalConfig()

	// map the peer list for the local node depending on the node's network
	for _, value := range ax.deviceCache {
		_, peerPort, err := net.SplitHostPort(value.LocalIp)
		if err != nil {
			ax.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v\n", err)
			continue
		}
		if value.PublicKey == ax.wireguardPubKey {
			// we found ourselves in the peer list
			continue
		}

		var peerHub wgPeerConfig
		// Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
		// This is the only peer a symmetric NAT node will get unless it also has a direct peering
		if !ax.relay && value.Relay {
			value.AllowedIps = append(value.AllowedIps, value.ChildPrefix...)
			ax.relayWgIP = relayIP
			peerHub = wgPeerConfig{
				value.PublicKey,
				value.LocalIp,
				relayAllowedIP,
				persistentKeepalive,
			}
			peers = append(peers, peerHub)
		}

		// Build the wg config for all peers if this node is the organization's hub-router.
		if ax.relay {
			// Config if this node is a relay
			value.AllowedIps = append(value.AllowedIps, value.ChildPrefix...)
			peer := wgPeerConfig{
				value.PublicKey,
				value.LocalIp,
				value.AllowedIps,
				persistentHubKeepalive,
			}
			peers = append(peers, peer)
			ax.logger.Infof("Peer Node Configuration - Peer AllowedIps [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIp IPv4 [ %s ] TunnelIp IPv6 [ %s ] Organization [ %s ]",
				value.AllowedIps,
				value.LocalIp,
				value.PublicKey,
				value.TunnelIp,
				value.TunnelIpV6,
				value.OrganizationId)
		}

		// If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
		// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
		if ax.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(value.ReflexiveIp4) && !value.Relay {
			directLocalPeerEndpointSocket := net.JoinHostPort(value.EndpointLocalAddressIp4, peerPort)
			ax.logger.Debugf("ICE candidate match for local address peering is [ %s ] with a STUN Address of [ %s ]", directLocalPeerEndpointSocket, value.ReflexiveIp4)
			// the symmetric NAT peer
			value.AllowedIps = append(value.AllowedIps, value.ChildPrefix...)
			peer := wgPeerConfig{
				value.PublicKey,
				directLocalPeerEndpointSocket,
				value.AllowedIps,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			ax.logger.Infof("Peer Configuration - Peer AllowedIps [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIp [ %s ] Organization [ %s ]",
				value.AllowedIps,
				directLocalPeerEndpointSocket,
				value.PublicKey,
				value.TunnelIp,
				value.OrganizationId)
		} else if !ax.symmetricNat && !value.SymmetricNat && !value.Relay {
			// the bulk of the peers will be added here except for local address peers. Endpoint sockets added here are likely
			// to be changed from the state discovered by the relay node if peering with nodes with NAT in between.
			// if the node itself (ax.symmetricNat) or the peer (value.SymmetricNat) is a
			// symmetric nat node, do not add peers as it will relay and not mesh
			value.AllowedIps = append(value.AllowedIps, value.ChildPrefix...)
			peer := wgPeerConfig{
				value.PublicKey,
				value.LocalIp,
				value.AllowedIps,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			ax.logger.Infof("Peer Configuration - Peer AllowedIps [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIp [ %s ] Organization [ %s ]",
				value.AllowedIps,
				value.LocalIp,
				value.PublicKey,
				value.TunnelIp,
				value.OrganizationId)
		}
	}
	ax.wgConfig.Peers = peers
}

// buildLocalConfig builds the configuration for the local interface
func (ax *Nexodus) buildLocalConfig() {
	var localInterface wgLocalConfig

	for _, value := range ax.deviceCache {
		// build the local interface configuration if this node is a Organization router
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
