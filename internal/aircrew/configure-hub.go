package aircrew

import (
	"fmt"
	"os"

	"github.com/redhat-et/apex/internal/messages"
	log "github.com/sirupsen/logrus"
)

const (
	hubPostUp   = "iptables -A FORWARD -i wg0 -o wg0 -j ACCEPT"
	hubPostDown = "iptables -D FORWARD -i wg0 -o wg0 -j ACCEPT"
)

// parseControlTowerHubWireguardConfig parse peerlisting to build the wireguard [Interface] and [Peer] sections
func (ac *Aircrew) parseControlTowerHubWireguardConfig(listenPort int, peerListing []messages.Peer) {

	var peers []wgPeerConfig
	var hubRouterIP string
	var localInterface wgLocalConfig
	var zonePrefix string

	for _, value := range peerListing {
		if value.PublicKey == ac.wireguardPubKey {
			ac.wireguardPubKeyInConfig = true
		}
		if value.HubRouter {
			hubRouterIP = value.AllowedIPs
			if ac.zone == value.ZoneID {
				zonePrefix = value.ZonePrefix
			}
		}
	}
	// zonePrefix will be empty if a hub-router is not defined in the zone
	// TODO: replace with an error message from the controller before it reaches this point
	if zonePrefix == "" {
		log.Error("This zone is a hub zone and requires a hub-router `--hub-router` node before provisioning spokes nodes")
		os.Exit(1)
	}
	if !ac.wireguardPubKeyInConfig {
		log.Printf("Public Key for this node %s was not found in the control tower update\n", ac.wireguardPubKey)
	}
	// Get a valid netmask from the zone prefix
	zoneCidr, err := ParseIPNet(zonePrefix)
	if err != nil {
		log.Errorf("Failed to parse a valid network the zone prefix %s: %v", zonePrefix, err)
		os.Exit(1)
	}
	zoneMask, _ := zoneCidr.Mask.Size()
	// Parse the [Peers] section of the wg config if this node is a zone-router
	for _, value := range peerListing {
		// Build the wg config for all peers
		if ac.hubRouter {
			// Config if the node is a bouncer hub
			if value.PublicKey != ac.wireguardPubKey {
				peer := wgPeerConfig{
					value.PublicKey,
					value.EndpointIP,
					value.AllowedIPs,
					persistentKeepalive,
				}
				peers = append(peers, peer)
				log.Printf("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
					value.AllowedIPs,
					value.EndpointIP,
					value.PublicKey,
					value.NodeAddress,
					value.ZoneID)
			}
		}
		// Build the wg config for all peers that are not zone routers (1 peer entry to the router)
		if !ac.hubRouter && value.HubRouter {
			if value.PublicKey != ac.wireguardPubKey {
				var allowedIPs string
				if value.ChildPrefix != "" {
					log.Warnf("Ignoring the child prefix since this is a hub zone")
				} else {
					allowedIPs = value.AllowedIPs
				}
				peer := wgPeerConfig{
					value.PublicKey,
					value.EndpointIP,
					fmt.Sprintf("%s/%d", hubRouterIP, zoneMask),
					persistentKeepalive,
				}
				peers = append(peers, peer)
				log.Printf("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
					allowedIPs,
					value.EndpointIP,
					value.PublicKey,
					value.NodeAddress,
					value.ZoneID)
			}
		}
		// Parse the [Interface] section of the wg config if this node is a zone-router
		if value.PublicKey == ac.wireguardPubKey && ac.hubRouter {
			localInterface = wgLocalConfig{
				ac.wireguardPvtKey,
				fmt.Sprintf("%s/%d", value.AllowedIPs, zoneMask),
				listenPort,
				false,
				hubPostUp,
				hubPostDown,
			}
			log.Printf("Local Node Configuration - Wireguard Local IP [ %s ] Wireguard Port [ %v ] HubZone Hub [ %t ]\n",
				localInterface.Address,
				listenPort,
				ac.hubRouter)
			// set the node unique local interface configuration
			ac.wgConfig.Interface = localInterface
		}
		// Parse the [Interface] section of the wg config if this node is not a zone router
		if value.PublicKey == ac.wireguardPubKey && !ac.hubRouter {
			localInterface = wgLocalConfig{
				ac.wireguardPvtKey,
				value.AllowedIPs,
				listenPort,
				false,
				"",
				"",
			}
			log.Printf("Local Node Configuration - Wireguard Local IP [ %s ] Wireguard Port [ %v ] HubZone Hub [ %t ]\n",
				localInterface.Address,
				listenPort,
				ac.hubRouter)
			// set the node unique local interface configuration
			ac.wgConfig.Interface = localInterface
		}
	}
	ac.wgConfig.Peer = peers
}
