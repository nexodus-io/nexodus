package nexodus

import (
	"net"
	"runtime"

	"go.uber.org/zap"
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
			relayIP = device.AllowedIPs[0]
		}
	}
	relayAllowedIP := []string{
		defaultOrganizationPrefixIPv4,
		defaultOrganizationPrefixIPv6,
	}

	// map the peer list for the local node depending on the node's network
	for _, value := range ax.deviceCache {
		_, peerPort, err := net.SplitHostPort(value.LocalIP)
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
			for _, prefix := range value.ChildPrefix {
				ax.addChildPrefixRoute(prefix)
				relayAllowedIP = append(relayAllowedIP, prefix)
			}
			ax.relayWgIP = relayIP
			peerHub = wgPeerConfig{
				value.PublicKey,
				value.LocalIP,
				relayAllowedIP,
				persistentKeepalive,
			}
			peers = append(peers, peerHub)
		}

		// Build the wg config for all peers if this node is the organization's hub-router.
		if ax.relay {
			// Config if this node is a relay
			for _, prefix := range value.ChildPrefix {
				ax.addChildPrefixRoute(prefix)
				value.AllowedIPs = append(value.AllowedIPs, prefix)
			}
			peer := wgPeerConfig{
				value.PublicKey,
				value.LocalIP,
				value.AllowedIPs,
				persistentHubKeepalive,
			}
			peers = append(peers, peer)
			ax.logger.Infof("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP IPv4 [ %s ] TunnelIP IPv6 [ %s ] Organization [ %s ]",
				value.AllowedIPs,
				value.LocalIP,
				value.PublicKey,
				value.TunnelIP,
				value.TunnelIpV6,
				value.OrganizationID)
		}

		// If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
		// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
		if ax.nodeReflexiveAddress == value.ReflexiveIPv4 && !value.Relay {
			directLocalPeerEndpointSocket := net.JoinHostPort(value.EndpointLocalAddressIPv4, peerPort)
			ax.logger.Debugf("ICE candidate match for local address peering is [ %s ] with a STUN Address of [ %s ]", directLocalPeerEndpointSocket, value.ReflexiveIPv4)
			// the symmetric NAT peer
			for _, prefix := range value.ChildPrefix {
				ax.addChildPrefixRoute(prefix)
				value.AllowedIPs = append(value.AllowedIPs, prefix)
			}
			peer := wgPeerConfig{
				value.PublicKey,
				directLocalPeerEndpointSocket,
				value.AllowedIPs,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			ax.logger.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Organization [ %s ]",
				value.AllowedIPs,
				directLocalPeerEndpointSocket,
				value.PublicKey,
				value.TunnelIP,
				value.OrganizationID)
		} else if !ax.symmetricNat && !value.SymmetricNat && !value.Relay {
			// the bulk of the peers will be added here except for local address peers. Endpoint sockets added here are likely
			// to be changed from the state discovered by the relay node if peering with nodes with NAT in between.
			// if the node itself (ax.symmetricNat) or the peer (value.SymmetricNat) is a
			// symmetric nat node, do not add peers as it will relay and not mesh
			for _, prefix := range value.ChildPrefix {
				ax.addChildPrefixRoute(prefix)
				value.AllowedIPs = append(value.AllowedIPs, prefix)
			}
			peer := wgPeerConfig{
				value.PublicKey,
				value.LocalIP,
				value.AllowedIPs,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			ax.logger.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Organization [ %s ]",
				value.AllowedIPs,
				value.LocalIP,
				value.PublicKey,
				value.TunnelIP,
				value.OrganizationID)
		}
	}
	ax.wgConfig.Peers = peers
	ax.buildLocalConfig()
}

// buildLocalConfig builds the configuration for the local interface
func (ax *Nexodus) buildLocalConfig() {
	var localInterface wgLocalConfig

	for _, value := range ax.deviceCache {
		// build the local interface configuration if this node is a Organization router
		if value.PublicKey == ax.wireguardPubKey {
			// if the local node address changed replace it on wg0
			if ax.TunnelIP != value.TunnelIP {
				ax.logger.Infof("New local Wireguard interface addresses assigned IPv4 [ %s ] IPv6 [ %s ]", value.TunnelIP, value.TunnelIpV6)
				if runtime.GOOS == Linux.String() && linkExists(ax.tunnelIface) {
					if err := delLink(ax.tunnelIface); err != nil {
						ax.logger.Infof("Failed to delete %s: %v", ax.tunnelIface, err)
					}
				}
			}
			ax.TunnelIP = value.TunnelIP
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

// relayIpTables iptables for the relay node
func relayIpTables(logger *zap.SugaredLogger, dev string) {
	_, err := RunCommand("iptables", "-A", "FORWARD", "-i", dev, "-j", "ACCEPT")
	if err != nil {
		logger.Debugf("the relay node v4 iptables rule was not added: %v", err)
	}
	_, err = RunCommand("ip6tables", "-A", "FORWARD", "-i", dev, "-j", "ACCEPT")
	if err != nil {
		logger.Debugf("tthe relay node v6 ip6tables rule was not added: %v", err)
	}
}
