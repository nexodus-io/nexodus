package nexodus

import (
	wg "github.com/nexodus-io/nexodus/internal/nexodus/wireguard"
)

// buildPeersConfig builds the peer configuration based off peer cache and peer listings from the controller
func (nexr *NexRelay) buildPeersConfig() {

	var peers []wg.WgPeerConfig
	var relayIP string
	//var localInterface wgLocalConfig
	var orgPrefix string
	var hubOrg bool
	var err error

	for _, device := range nexr.nex.deviceCache {
		if device.PublicKey == nexr.wg.WireguardPubKey {
			nexr.wg.WireguardPubKeyInConfig = true
		}
		if device.Relay {
			relayIP = device.AllowedIPs[0]
			if nexr.nex.organization == device.OrganizationID {
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
		nexr.logger.Errorf("there is no hub router detected in this organization, please add one using `--hub-router`")
		return
	}

	if err != nil {
		nexr.logger.Errorf("invalid hub router network found: %v", err)
	}
	// map the peer list for the local node depending on the node's network
	for _, value := range nexr.nex.deviceCache {
		if err != nil {
			nexr.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v\n", err)
			continue
		}
		if value.PublicKey == nexr.wg.WireguardPubKey {
			// we found ourself in the peer list
			continue
		}

		// Build the wg config for all peers if this node is the organization's hub-router.
		// Config if the node is a relay
		for _, prefix := range value.ChildPrefix {
			nexr.wg.AddChildPrefixRoute(prefix)
			value.AllowedIPs = append(value.AllowedIPs, prefix)
		}
		peer := wg.WgPeerConfig{
			PublicKey:           value.PublicKey,
			Endpoint:            value.LocalIP,
			AllowedIPs:          value.AllowedIPs,
			PersistentKeepAlive: wg.PersistentHubKeepalive,
		}
		peers = append(peers, peer)
		nexr.logger.Infof("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Organization [ %s ]",
			value.AllowedIPs,
			value.LocalIP,
			value.PublicKey,
			value.TunnelIP,
			value.OrganizationID)

		if !value.SymmetricNat && !value.Relay {
			// the bulk of the peers will be added here except for local address peers. Endpoint sockets added here are likely
			// to be changed from the state discovered by the relay node if peering with nodes with NAT in between.
			// if the node itself (ax.symmetricNat) or the peer (value.SymmetricNat) is a
			// symmetric nat node, do not add peers as it will relay and not mesh
			for _, prefix := range value.ChildPrefix {
				nexr.wg.AddChildPrefixRoute(prefix)
				value.AllowedIPs = append(value.AllowedIPs, prefix)
			}
			peer := wg.WgPeerConfig{
				PublicKey:           value.PublicKey,
				Endpoint:            value.LocalIP,
				AllowedIPs:          value.AllowedIPs,
				PersistentKeepAlive: wg.PersistentKeepalive,
			}
			peers = append(peers, peer)
			nexr.logger.Infof("Peer Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] TunnelIP [ %s ] Organization [ %s ]",
				value.AllowedIPs,
				value.LocalIP,
				value.PublicKey,
				value.TunnelIP,
				value.OrganizationID)
		}
	}
	nexr.wg.Peers = peers
	nexr.buildLocalConfig()
}

// buildLocalConfig builds the configuration for the local interface
func (nexr *NexRelay) buildLocalConfig() {

	for _, value := range nexr.nex.deviceCache {
		// build the local interface configuration if this node is a Organization router
		if value.PublicKey == nexr.wg.WireguardPubKey {
			// if the local node address changed replace it on wg0
			if nexr.wg.WgLocalAddress != value.TunnelIP {
				nexr.logger.Infof("New local Wireguard interface address assigned: %s", value.TunnelIP)
				nexr.wg.RemoveExistingInterface()
			}
			nexr.wg.WgLocalAddress = value.TunnelIP
			nexr.logger.Debugf("Local Node Configuration - Wireguard IP [ %s ]", nexr.wg.WireguardPubKeyInConfig)
		}
	}
}
