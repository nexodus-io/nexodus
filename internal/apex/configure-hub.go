package apex

import (
	"fmt"
	"net"
	"os"

	log "github.com/sirupsen/logrus"
)

const (
	hubPostUp   = "iptables -A FORWARD -i wg0 -o wg0 -j ACCEPT"
	hubPostDown = "iptables -D FORWARD -i wg0 -o wg0 -j ACCEPT"
)

// parseHubWireguardConfig parse peerlisting to build the wireguard [Interface] and [Peer] sections
func (ax *Apex) parseHubWireguardConfig(listenPort int, peerListing []Peer) {

	var peers []wgPeerConfig
	var hubRouterIP string
	var hubRouterEndpointIP string
	var localInterface wgLocalConfig
	var zonePrefix string
	var err error

	for _, value := range peerListing {
		pubkey := ax.peerMap[value.ID]
		if pubkey == ax.wireguardPubKey {
			ax.wireguardPubKeyInConfig = true
		}
		if value.HubRouter {
			hubRouterIP = value.AllowedIPs
			hubRouterEndpointIP, _, err = net.SplitHostPort(value.EndpointIP)
			if err != nil {
				log.Errorf("failed to split host:port endpoint pair: %v", err)
			}
			if ax.zone == value.ZoneID {
				zonePrefix = value.ZonePrefix
			}
		}
	}
	// zonePrefix will be empty if a hub-router is not defined in the zone
	// TODO: replace with an error message from the controller before it reaches this point
	if zonePrefix == "" {
		log.Error("this zone is a hub zone and requires a hub-router `--hub-router` node before provisioning spokes nodes")
		os.Exit(1)
	}
	if !ax.wireguardPubKeyInConfig {
		log.Printf("Public Key for this node %s was not found in the controller update\n", ax.wireguardPubKey)
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
	if err != nil {
		log.Errorf("invalid hub router network found: %v", err)
	}
	var peerEndpoints []string
	var reachablePeers []string
	if !ax.hubRouter {
		for _, value := range peerListing {
			peerIP, _, err := net.SplitHostPort(value.EndpointIP)
			if err != nil {
				log.Errorf("failed to split host:port endpoint pair: %v", err)
			}
			peerEndpoints = append(peerEndpoints, peerIP)
		}
	}
	// basic discovery of what endpoints are reachable from the spoke peer that
	// determines whether to drain traffic to the hub or build a p2p peering
	// TODO: replace with a more in depth discovery than simple reachability
	reachablePeers = probePeers(peerEndpoints)
	// remove the hub router from the list since connectivity is required(ish)
	reachablePeers = removeElement(reachablePeers, hubRouterEndpointIP)
	// remove the node the agent is running on from the peer list (eg. don't peer to yourself)
	reachablePeers = removeElement(reachablePeers, ax.localEndpointIP)
	log.Debugf("reachable endpoint peers by this node are %s", reachablePeers)

	// Parse the [Peers] section of the wg config if this node is a zone-router
	for _, value := range peerListing {
		pubkey := ax.peerMap[value.ID]
		// Build the wg config for all peers for the hub-router node
		if ax.hubRouter {
			// Config if the node is a bouncer hub
			if pubkey != ax.wireguardPubKey {
				peer := wgPeerConfig{
					pubkey,
					value.EndpointIP,
					value.AllowedIPs,
					persistentHubKeepalive,
				}
				peers = append(peers, peer)
				log.Printf("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
					value.AllowedIPs,
					value.EndpointIP,
					pubkey,
					value.NodeAddress,
					value.ZoneID)
			}
		}
		var peerHub wgPeerConfig
		// Build the wg config for all peers that are not zone routers (1 peer entry to the router)
		if !ax.hubRouter && value.HubRouter {
			if pubkey != ax.wireguardPubKey {
				//var allowedIPs string
				if value.ChildPrefix != "" {
					log.Warnf("Ignoring the child prefix since this is a hub zone")
				}
				ax.hubRouterWgIP = hubRouterIP
				peerHub = wgPeerConfig{
					pubkey,
					value.EndpointIP,
					hubRouterNetAddress,
					persistentKeepalive,
				}
				peers = append(peers, peerHub)
			}
		}

		// Peers that are reachable for spokes
		peerIP, _, err := net.SplitHostPort(value.EndpointIP)
		if err != nil {
			log.Errorf("failed to split host:port endpoint pair: %v", err)
		}
		if isReachable(reachablePeers, peerIP) {
			peer := wgPeerConfig{
				pubkey,
				value.EndpointIP,
				value.AllowedIPs,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			log.Printf("Spoke Node Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
				value.AllowedIPs,
				value.EndpointIP,
				pubkey,
				value.NodeAddress,
				value.ZoneID)
		}
		// Parse the [Interface] section of the wg config if this node is a zone-router
		if pubkey == ax.wireguardPubKey && ax.hubRouter {
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				fmt.Sprintf("%s/%d", value.AllowedIPs, zoneMask),
				listenPort,
				false,
				hubPostUp,
				hubPostDown,
			}
			log.Printf("Local Node Configuration - Wireguard IP [ %s ] Wireguard Port [ %v ] Hub Router [ %t ]\n",
				localInterface.Address,
				listenPort,
				ax.hubRouter)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
		// Parse the [Interface] section of the wg config if this node is not a zone router
		if pubkey == ax.wireguardPubKey && !ax.hubRouter {
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				value.AllowedIPs,
				listenPort,
				false,
				"",
				"",
			}
			log.Printf("Local Node Configuration - Wireguard IP [ %s ] Wireguard Port [ %v ] Hub Router [ %t ]\n",
				localInterface.Address,
				listenPort,
				ax.hubRouter)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
	}
	ax.wgConfig.Peer = peers
}

func isReachable(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func removeElement(items []string, item string) []string {
	var updatedSlice []string
	for _, i := range items {
		if i != item {
			updatedSlice = append(updatedSlice, i)
		}
	}
	return updatedSlice
}
