package apex

import (
	"fmt"
	"net"
	"os"

	log "github.com/sirupsen/logrus"
)

// parseHubWireguardConfig parse peerlisting to build the wireguard local interface and peers
func (ax *Apex) parseHubWireguardConfig(listenPort int) {

	var peers []wgPeerConfig
	var hubRouterIP string
	var localInterface wgLocalConfig
	var zonePrefix string
	var err error

	for _, value := range ax.peerCache {
		if value.HubRouter {
			hubRouterIP = value.AllowedIPs[0]
			if ax.zone == value.ZoneID {
				zonePrefix = value.ZonePrefix
			}
		}
	}
	// zonePrefix will be empty if a hub-router is not defined in the zone
	if zonePrefix == "" {
		log.Error("this zone is a hub zone and requires a hub-router `--hub-router` node before provisioning spokes nodes")
		os.Exit(1)
	}
	// Get a valid netmask from the zone prefix
	zoneCidr, err := ParseIPNet(zonePrefix)
	if err != nil {
		log.Errorf("failed to parse a valid network the zone prefix %s: %v", zonePrefix, err)
		os.Exit(1)
	}
	zoneMask, _ := zoneCidr.Mask.Size()
	hubRouterNetAddress := fmt.Sprintf("%s/%d", hubRouterIP, zoneMask)
	hubRouterNetAddress, err = parseNetworkStr(hubRouterNetAddress)
	hubRouterAllowedIP := []string{hubRouterNetAddress}
	if err != nil {
		log.Errorf("invalid hub router network found: %v", err)
	}
	// map the peer list for the local node depending on the node's network
	for _, value := range ax.peerCache {
		_, peerPort, err := net.SplitHostPort(value.EndpointIP)
		if err != nil {
			log.Debugf("failed parse the endpoint address for node (likely still converging) : %v\n", err)
			continue
		}

		pubkey := ax.keyCache[value.DeviceID]
		var peerHub wgPeerConfig
		// Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
		// This is the only peer a symmetric NAT node will get unless it also has a direct peering
		if !ax.hubRouter && value.HubRouter {
			if pubkey != ax.wireguardPubKey {
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
		}
		// Build the wg config for all peers if this node is the zone's hub-router.
		if ax.hubRouter {
			// Config if the node is a relay
			if pubkey != ax.wireguardPubKey {
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
		}
		// if both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
		if ax.nodeReflexiveAddress == value.ReflexiveIPv4 {
			if pubkey != ax.wireguardPubKey {
				directLocalPeerEndpointSocket := fmt.Sprintf("%s:%s", value.EnpointLocalAddressIPv4, peerPort)
				log.Infof("ICE candidate match for local address peering is [ %s ] with a STUN Address of [ %s ]", directLocalPeerEndpointSocket, value.ReflexiveIPv4)
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
			}
		} else {
			// the bulk of the peers will be added here except for local address peers. Endpoint sockets added here are likely
			// to be changed from the state discovered by the relay node if peering with nodes with NAT in between.
			if pubkey != ax.wireguardPubKey {
				// if the node itself (ax.symmetricNat) or the peer (value.SymmetricNat) is a
				// symmetric nat node, do not add peers as it will relay and not mesh
				if !ax.symmetricNat {
					if !value.SymmetricNat {
						if !value.HubRouter {
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
				}
			}
		}
	}

	// Deal with the wireguard local interface
	for _, value := range ax.peerCache {
		pubkey := ax.keyCache[value.DeviceID]
		if pubkey == ax.wireguardPubKey && ax.hubRouter {
			if ax.wgLocalAddress != value.NodeAddress {
				log.Infof("New local interface address assigned %s", value.NodeAddress)
				if ax.os == Linux.String() && linkExists(wgIface) {
					if err = delLink(wgIface); err != nil {
						log.Infof("Failed to delete %s: %v", wgIface, err)
					}
				}
			}
			ax.wgLocalAddress = value.NodeAddress
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				listenPort,
			}
			log.Infof("Local Node Configuration - Wireguard IP [ %s ] Wireguard Port [ %v ] Hub Router [ %t ]\n",
				ax.wgLocalAddress,
				listenPort,
				ax.hubRouter)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
		// Parse the [Interface] section of the wg config if this node is not a zone router
		if pubkey == ax.wireguardPubKey && !ax.hubRouter {
			// if the local node address changed replace it on wg0
			if ax.wgLocalAddress != value.NodeAddress {
				log.Infof("New local interface address assigned %s", value.NodeAddress)
				if ax.os == Linux.String() && linkExists(wgIface) {
					if err = delLink(wgIface); err != nil {
						log.Infof("Failed to delete %s: %v", wgIface, err)
					}
				}
			}
			ax.wgLocalAddress = value.NodeAddress
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				listenPort,
			}
			log.Infof("Local Node Configuration - Wireguard IP [ %s ] Wireguard Port [ %v ] Hub Router [ %t ]\n",
				ax.wgLocalAddress,
				listenPort,
				ax.hubRouter)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
	}
	ax.wgConfig.Peers = peers
}

// hubRouterIpTables iptables for the relay node
func hubRouterIpTables() {
	_, err := RunCommand("iptables", "-A", "FORWARD", "-i", wgIface, "-j", "ACCEPT")
	if err != nil {
		log.Debugf("the hub router iptable rule was not added: %v", err)
	}
}

func (ax *Apex) addChildPrefixRoute(ChildPrefix string) {
	if ax.os == Linux.String() && routeExists(ChildPrefix) {
		log.Debugf("unable to add the child-prefix route [ %s ] as it already exists on this linux host", ChildPrefix)
		return
	}
	if ax.os == Linux.String() {
		if err := addLinuxChildPrefixRoute(ChildPrefix); err != nil {
			log.Infof("error adding the child prefix route: %v", err)
		}
	}
	// add osx child prefix
	if ax.os == Darwin.String() {
		if err := addDarwinChildPrefixRoute(ChildPrefix); err != nil {
			// TODO: setting to debug until the child prefix is looked up on Darwin
			log.Debugf("error adding the child prefix route: %v", err)
		}
	}
}
