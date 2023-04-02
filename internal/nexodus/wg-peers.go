package nexodus

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"go.uber.org/zap"
)

// buildPeersConfig builds the peer configuration based off peer cache and peer listings from the controller
func (ax *Nexodus) buildPeersConfig() {

	var peers []wgPeerConfig
	var relayIP string
	//var localInterface wgLocalConfig
	var orgPrefix string
	var hubOrg bool
	var err error

	for _, device := range ax.deviceCache {
		if device.PublicKey == ax.wireguardPubKey {
			ax.wireguardPubKeyInConfig = true
		}
		if device.Relay {
			relayIP = device.AllowedIPs[0]
			if ax.organization == device.OrganizationID {
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
		ax.logger.Errorf("there is no hub router detected in this organization, please add one using `--hub-router`")
		return
	}
	// Get a valid netmask from the organization prefix
	var relayAllowedIP []string
	if hubOrg {
		orgCidr, err := ParseIPNet(orgPrefix)
		if err != nil {
			ax.logger.Errorf("failed to parse a valid network organization prefix cidr %s: %v", orgPrefix, err)
			os.Exit(1)
		}
		orgMask, _ := orgCidr.Mask.Size()
		relayNetAddress := fmt.Sprintf("%s/%d", relayIP, orgMask)
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
			// Config if the node is a relay
			value.AllowedIPs = append(value.AllowedIPs, value.ChildPrefix...)
			peer := wgPeerConfig{
				value.PublicKey,
				value.LocalIP,
				value.AllowedIPs,
				persistentHubKeepalive,
			}
			peers = append(peers, peer)
			ax.logger.Infof("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Organization [ %s ]",
				value.AllowedIPs,
				value.LocalIP,
				value.PublicKey,
				value.TunnelIP,
				value.OrganizationID)
		}

		// If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
		// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
		if ax.nodeReflexiveAddress == value.ReflexiveIPv4 && !value.Relay {
			directLocalPeerEndpointSocket := net.JoinHostPort(value.EndpointLocalAddressIPv4, peerPort)
			ax.logger.Debugf("ICE candidate match for local address peering is [ %s ] with a STUN Address of [ %s ]", directLocalPeerEndpointSocket, value.ReflexiveIPv4)
			// the symmetric NAT peer
			value.AllowedIPs = append(value.AllowedIPs, value.ChildPrefix...)
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
			value.AllowedIPs = append(value.AllowedIPs, value.ChildPrefix...)
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
			if ax.wgLocalAddress != value.TunnelIP {
				ax.logger.Infof("New local Wireguard interface address assigned: %s", value.TunnelIP)
				if runtime.GOOS == Linux.String() && linkExists(ax.tunnelIface) {
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
			ax.logger.Debugf("Local Node Configuration - Wireguard IP [ %s ]", ax.wgLocalAddress)
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
