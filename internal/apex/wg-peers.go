package apex

import (
	"fmt"
	"net"
	"os"

	"go.uber.org/zap"
)

// buildPeersConfig builds the peer configuration based off peer cache and peer listings from the controller
func (ax *Apex) buildPeersConfig() {

	var peers []wgPeerConfig
	var relayIP string
	//var localInterface wgLocalConfig
	var zonePrefix string
	var hubZone bool
	var err error

	for _, device := range ax.deviceCache {
		if device.PublicKey == ax.wireguardPubKey {
			ax.wireguardPubKeyInConfig = true
		}
		if device.Relay {
			relayIP = device.AllowedIPs[0]
			if ax.zone == device.OrganizationID {
				zonePrefix = device.OrganizationPrefix
			}
		}
	}
	// zonePrefix will be empty if a hub-router is not defined in the zone
	if zonePrefix != "" {
		hubZone = true
	}
	// if this is a zone router but does not have a relay node joined yet throw an error
	if relayIP == "" && hubZone {
		ax.logger.Errorf("there is no hub router detected in this zone, please add one using `--hub-router`")
		return
	}
	// Get a valid netmask from the zone prefix
	var relayAllowedIP []string
	if hubZone {
		zoneCidr, err := ParseIPNet(zonePrefix)
		if err != nil {
			ax.logger.Errorf("failed to parse a valid network zone prefix cidr %s: %v", zonePrefix, err)
			os.Exit(1)
		}
		zoneMask, _ := zoneCidr.Mask.Size()
		relayNetAddress := fmt.Sprintf("%s/%d", relayIP, zoneMask)
		relayNetAddress, err = parseNetworkStr(relayNetAddress)
		if err != nil {
			ax.logger.Errorf("failed to parse a valid hub router prefix from %s: %v", relayNetAddress, err)
		}
		relayAllowedIP = []string{relayNetAddress}
	}

	if err != nil {
		ax.logger.Errorf("invalid hub router network found: %v", err)
	}
	// map the peer list for the local node depending on the node's network
	for _, value := range ax.deviceCache {
		_, peerPort, err := net.SplitHostPort(value.LocalIP)
		if err != nil {
			ax.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v\n", err)
			continue
		}
		if value.PublicKey == ax.wireguardPubKey {
			// we found ourself in the peer list
			continue
		}

		var peerHub wgPeerConfig
		// Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
		// This is the only peer a symmetric NAT node will get unless it also has a direct peering
		if !ax.relay && value.Relay {
			for _, prefix := range value.ChildPrefix {
				ax.addChildPrefixRoute(prefix)
				value.AllowedIPs = append(value.AllowedIPs, prefix)
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

		// Build the wg config for all peers if this node is the zone's hub-router.
		if ax.relay {
			// Config if the node is a relay
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
			ax.logger.Infof("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Zone [ %s ]\n",
				value.AllowedIPs,
				value.LocalIP,
				value.PublicKey,
				value.TunnelIP,
				value.OrganizationID)
		}

		// if both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
		if ax.nodeReflexiveAddress == value.ReflexiveIPv4 {
			directLocalPeerEndpointSocket := net.JoinHostPort(value.EndpointLocalAddressIPv4, peerPort)
			ax.logger.Infof("ICE candidate match for local address peering is [ %s ] with a STUN Address of [ %s ]", directLocalPeerEndpointSocket, value.ReflexiveIPv4)
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
			ax.logger.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Zone [ %s ]\n",
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
			ax.logger.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Zone [ %s ]\n",
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
func (ax *Apex) buildLocalConfig() {
	var localInterface wgLocalConfig

	for _, value := range ax.deviceCache {
		// build the local interface configuration if this node is a zone router
		if value.PublicKey == ax.wireguardPubKey {
			// if the local node address changed replace it on wg0
			if ax.wgLocalAddress != value.TunnelIP {
				ax.logger.Infof("New local interface address assigned %s", value.TunnelIP)
				if ax.os == Linux.String() && linkExists(ax.tunnelIface) {
					if err := delLink(ax.tunnelIface); err != nil {
						ax.logger.Infof("Failed to delete %s: %v", ax.tunnelIface, err)
					}
				}
			}
			ax.wgLocalAddress = value.TunnelIP
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				ax.listenPort,
			}
			ax.logger.Infof("Local Node Configuration - Wireguard IP [ %s ] Wireguard Port [ %v ] Hub Router [ %t ]\n",
				ax.wgLocalAddress,
				ax.listenPort,
				ax.relay)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
	}
}

// relayIpTables iptables for the relay node
func relayIpTables(logger *zap.SugaredLogger, dev string) {
	_, err := RunCommand("iptables", "-A", "FORWARD", "-i", dev, "-j", "ACCEPT")
	if err != nil {
		logger.Debugf("the hub router iptables rule was not added: %v", err)
	}
}
