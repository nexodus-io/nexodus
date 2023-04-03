package nexodus

import (
	"fmt"
	"net"
	"os"

	wg "github.com/nexodus-io/nexodus/internal/nexodus/wireguard"
)

// buildPeersConfig builds the peer configuration based off peer cache and peer listings from the controller
func (nexa *NexAgent) buildPeersConfig() {

	var peers []wg.WgPeerConfig
	var relayIP string
	//var localInterface wgLocalConfig
	var orgPrefix string
	var hubOrg bool
	var err error

	for _, device := range nexa.nex.deviceCache {
		if device.PublicKey == nexa.wg.WireguardPubKey {
			nexa.wg.WireguardPubKeyInConfig = true
		}
		if device.Relay {
			relayIP = device.AllowedIPs[0]
			if nexa.nex.organization == device.OrganizationID {
				orgPrefix = device.OrganizationPrefix
			}
		}
	}
	// orgPrefix will be empty if a hub-router is not defined in the organization
	if orgPrefix != "" {
		hubOrg = true
	}
	// if this is a org router but does not have a relay node joined yet throw an error
	if relayIP == "" && hubOrg {
		nexa.logger.Errorf("there is no hub router detected in this organization, please add one using `--hub-router`")
		return
	}
	// Get a valid netmask from the organization prefix
	var relayAllowedIP []string
	if hubOrg {
		orgCidr, err := parseIPNet(orgPrefix)
		if err != nil {
			nexa.logger.Errorf("failed to parse a valid network organization prefix cidr %s: %v", orgPrefix, err)
			os.Exit(1)
		}
		orgMask, _ := orgCidr.Mask.Size()
		relayNetAddress := fmt.Sprintf("%s/%d", relayIP, orgMask)
		relayNetAddress, err = parseNetworkStr(relayNetAddress)
		if err != nil {
			nexa.logger.Errorf("failed to parse a valid hub router prefix from %s: %v", relayNetAddress, err)
		}
		relayAllowedIP = []string{relayNetAddress}
	}

	if err != nil {
		nexa.logger.Errorf("invalid hub router network found: %v", err)
	}
	// map the peer list for the local node depending on the node's network
	for _, value := range nexa.nex.deviceCache {
		_, peerPort, err := net.SplitHostPort(value.LocalIP)
		if err != nil {
			nexa.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v\n", err)
			continue
		}
		if value.PublicKey == nexa.wg.WireguardPubKey {
			// we found ourself in the peer list
			continue
		}

		var peerHub wg.WgPeerConfig
		// Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
		// This is the only peer a symmetric NAT node will get unless it also has a direct peering
		if value.Relay {
			for _, prefix := range value.ChildPrefix {
				nexa.wg.AddChildPrefixRoute(prefix)
				relayAllowedIP = append(relayAllowedIP, prefix)
			}
			nexa.nex.relayWgIP = relayIP
			peerHub = wg.WgPeerConfig{
				PublicKey:           value.PublicKey,
				Endpoint:            value.LocalIP,
				AllowedIPs:          relayAllowedIP,
				PersistentKeepAlive: wg.PersistentKeepalive,
			}
			peers = append(peers, peerHub)
		}

		// If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
		// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
		if nexa.nex.nodeReflexiveAddress == value.ReflexiveIPv4 && !value.Relay {
			directLocalPeerEndpointSocket := net.JoinHostPort(value.EndpointLocalAddressIPv4, peerPort)
			nexa.logger.Debugf("ICE candidate match for local address peering is [ %s ] with a STUN Address of [ %s ]", directLocalPeerEndpointSocket, value.ReflexiveIPv4)
			// the symmetric NAT peer
			for _, prefix := range value.ChildPrefix {
				nexa.wg.AddChildPrefixRoute(prefix)
				value.AllowedIPs = append(value.AllowedIPs, prefix)
			}
			peer := wg.WgPeerConfig{
				PublicKey:           value.PublicKey,
				Endpoint:            directLocalPeerEndpointSocket,
				AllowedIPs:          value.AllowedIPs,
				PersistentKeepAlive: wg.PersistentKeepalive,
			}
			peers = append(peers, peer)
			nexa.logger.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Organization [ %s ]",
				value.AllowedIPs,
				directLocalPeerEndpointSocket,
				value.PublicKey,
				value.TunnelIP,
				value.OrganizationID)
		} else if !nexa.symmetricNat && !value.SymmetricNat && !value.Relay {
			// the bulk of the peers will be added here except for local address peers. Endpoint sockets added here are likely
			// to be changed from the state discovered by the relay node if peering with nodes with NAT in between.
			// if the node itself (ax.symmetricNat) or the peer (value.SymmetricNat) is a
			// symmetric nat node, do not add peers as it will relay and not mesh
			for _, prefix := range value.ChildPrefix {
				nexa.wg.AddChildPrefixRoute(prefix)
				value.AllowedIPs = append(value.AllowedIPs, prefix)
			}
			peer := wg.WgPeerConfig{
				PublicKey:           value.PublicKey,
				Endpoint:            value.LocalIP,
				AllowedIPs:          value.AllowedIPs,
				PersistentKeepAlive: wg.PersistentKeepalive,
			}
			peers = append(peers, peer)
			nexa.logger.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Organization [ %s ]",
				value.AllowedIPs,
				value.LocalIP,
				value.PublicKey,
				value.TunnelIP,
				value.OrganizationID)
		}
	}
	nexa.wg.Peers = peers
	nexa.buildLocalConfig()
}

// buildLocalConfig builds the configuration for the local interface
func (nexa *NexAgent) buildLocalConfig() {

	for _, value := range nexa.nex.deviceCache {
		// build the local interface configuration if this node is a Organization router
		if value.PublicKey == nexa.wg.WireguardPubKey {
			// if the local node address changed replace it on wg0
			if nexa.wg.WgLocalAddress != value.TunnelIP {
				nexa.logger.Infof("New local Wireguard interface address assigned: %s", value.TunnelIP)
				nexa.wg.RemoveExistingInterface()
			}
			nexa.wg.WgLocalAddress = value.TunnelIP
			nexa.logger.Debugf("Local Node Configuration - Wireguard IP [ %s ]", nexa.wg.WireguardPubKeyInConfig)
		}
	}
}
