package apex

import (
	"fmt"
	"net"
	"os"

	log "github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

// buildPeersConfig builds the peer configuration based off peer cache and peer listings from the controller
func (ax *Apex) buildPeersConfig() {

	var peers []wgPeerConfig
	var hubRouterIP string
	//var localInterface wgLocalConfig
	var zonePrefix string
	var hubZone bool
	var err error

	for _, peer := range ax.peerCache {
		var pubkey string
		var ok bool
		if pubkey, ok = ax.keyCache[peer.DeviceID]; !ok {
			device, err := ax.client.GetDevice(peer.DeviceID)
			if err != nil {
				ax.logger.Warnf("unable to get device %s: %s", peer.DeviceID, err)
			}
			ax.keyCache[peer.DeviceID] = device.PublicKey
			pubkey = device.PublicKey
		}

		if pubkey == ax.wireguardPubKey {
			ax.wireguardPubKeyInConfig = true
		}

		if peer.HubRouter {
			hubRouterIP = peer.AllowedIPs[0]
			if ax.zone == peer.ZoneID {
				zonePrefix = peer.ZonePrefix
			}
		}
	}
	// zonePrefix will be empty if a hub-router is not defined in the zone
	if zonePrefix != "" {
		hubZone = true
	}
	// if this is a zone router but does not have a relay node joined yet throw an error
	if hubRouterIP == "" && hubZone {
		ax.logger.Errorf("there is no hub router detected in this zone, please add one using `--hub-router`")
		return
	}
	// Get a valid netmask from the zone prefix
	var hubRouterAllowedIP []string
	if hubZone {
		zoneCidr, err := ParseIPNet(zonePrefix)
		if err != nil {
			ax.logger.Errorf("failed to parse a valid network zone prefix cidr %s: %v", zonePrefix, err)
			os.Exit(1)
		}
		zoneMask, _ := zoneCidr.Mask.Size()
		hubRouterNetAddress := fmt.Sprintf("%s/%d", hubRouterIP, zoneMask)
		hubRouterNetAddress, err = parseNetworkStr(hubRouterNetAddress)
		if err != nil {
			ax.logger.Errorf("failed to parse a valid hub router prefix from %s: %v", hubRouterNetAddress, err)
		}
		hubRouterAllowedIP = []string{hubRouterNetAddress}
	}

	if err != nil {
		ax.logger.Errorf("invalid hub router network found: %v", err)
	}
	// map the peer list for the local node depending on the node's network
	for _, value := range ax.peerCache {
		_, peerPort, err := net.SplitHostPort(value.EndpointIP)
		if err != nil {
			ax.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v\n", err)
			continue
		}

		pubkey := ax.keyCache[value.DeviceID]
		if pubkey == ax.wireguardPubKey {
			// we found ourself in the peer list
			continue
		}

		var peerHub wgPeerConfig
		// Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
		// This is the only peer a symmetric NAT node will get unless it also has a direct peering
		if !ax.hubRouter && value.HubRouter {
			if value.ChildPrefix != "" {
				ax.addChildPrefixRoute(value.ChildPrefix)
				value.AllowedIPs = append(value.AllowedIPs, value.ChildPrefix)
			}
			ax.hubRouterWgIP = hubRouterIP
			peerHub = wgPeerConfig{
				pubkey,
				value.EndpointIP,
				hubRouterAllowedIP,
				persistentKeepalive,
			}
			peers = append(peers, peerHub)
		}

		// Build the wg config for all peers if this node is the zone's hub-router.
		if ax.hubRouter {
			// Config if the node is a relay
			if value.ChildPrefix != "" {
				ax.addChildPrefixRoute(value.ChildPrefix)
				value.AllowedIPs = append(value.AllowedIPs, value.ChildPrefix)
			}
			peer := wgPeerConfig{
				pubkey,
				value.EndpointIP,
				value.AllowedIPs,
				persistentHubKeepalive,
			}
			peers = append(peers, peer)
			log.Infof("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
				value.AllowedIPs,
				value.EndpointIP,
				pubkey,
				value.NodeAddress,
				value.ZoneID)
		}

		// if both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
		if ax.nodeReflexiveAddress == value.ReflexiveIPv4 {
			directLocalPeerEndpointSocket := fmt.Sprintf("%s:%s", value.EndpointLocalAddressIPv4, peerPort)
			ax.logger.Infof("ICE candidate match for local address peering is [ %s ] with a STUN Address of [ %s ]", directLocalPeerEndpointSocket, value.ReflexiveIPv4)
			// the symmetric NAT peer
			if value.ChildPrefix != "" {
				ax.addChildPrefixRoute(value.ChildPrefix)
				value.AllowedIPs = append(value.AllowedIPs, value.ChildPrefix)
			}
			peer := wgPeerConfig{
				pubkey,
				directLocalPeerEndpointSocket,
				value.AllowedIPs,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			log.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
				value.AllowedIPs,
				directLocalPeerEndpointSocket,
				pubkey,
				value.NodeAddress,
				value.ZoneID)
		} else if !ax.symmetricNat && !value.SymmetricNat && !value.HubRouter {
			// the bulk of the peers will be added here except for local address peers. Endpoint sockets added here are likely
			// to be changed from the state discovered by the relay node if peering with nodes with NAT in between.
			// if the node itself (ax.symmetricNat) or the peer (value.SymmetricNat) is a
			// symmetric nat node, do not add peers as it will relay and not mesh
			if value.ChildPrefix != "" {
				ax.addChildPrefixRoute(value.ChildPrefix)
				value.AllowedIPs = append(value.AllowedIPs, value.ChildPrefix)
			}
			peer := wgPeerConfig{
				pubkey,
				value.EndpointIP,
				value.AllowedIPs,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			log.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
				value.AllowedIPs,
				value.EndpointIP,
				pubkey,
				value.NodeAddress,
				value.ZoneID)
		}
	}
	ax.wgConfig.Peers = peers
	ax.buildLocalConfig()
}

// buildLocalConfig builds the configuration for the local interface
func (ax *Apex) buildLocalConfig() {
	var localInterface wgLocalConfig

	for _, value := range ax.peerCache {
		pubkey := ax.keyCache[value.DeviceID]
		// build the local interface configuration if this node is a zone router
		if pubkey == ax.wireguardPubKey {
			// if the local node address changed replace it on wg0
			if ax.wgLocalAddress != value.NodeAddress {
				ax.logger.Infof("New local interface address assigned %s", value.NodeAddress)
				if ax.os == Linux.String() && linkExists(ax.tunnelIface) {
					if err := delLink(ax.tunnelIface); err != nil {
						ax.logger.Infof("Failed to delete %s: %v", ax.tunnelIface, err)
					}
				}
			}
			ax.wgLocalAddress = value.NodeAddress
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				ax.listenPort,
			}
			ax.logger.Infof("Local Node Configuration - Wireguard IP [ %s ] Wireguard Port [ %v ] Hub Router [ %t ]\n",
				ax.wgLocalAddress,
				ax.listenPort,
				ax.hubRouter)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
	}
}

// hubRouterIpTables iptables for the relay node
func hubRouterIpTables(logger *zap.SugaredLogger, dev string) {
	_, err := RunCommand("iptables", "-A", "FORWARD", "-i", dev, "-j", "ACCEPT")
	if err != nil {
		logger.Debugf("the hub router iptables rule was not added: %v", err)
	}
}
