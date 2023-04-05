package nexodus

import (
	"github.com/nexodus-io/nexodus/internal/api/public"
)

const (
	// wg keepalives are disabled and managed by the agent
	persistentKeepalive    = "0"
	persistentHubKeepalive = "0"
)

func (ax *Nexodus) DeployWireguardConfig(newPeers []public.ModelsDevice, firstTime bool) error {
	cfg := &wgConfig{
		Interface: ax.wgConfig.Interface,
		Peers:     ax.wgConfig.Peers,
	}

	if ax.TunnelIP != ax.getIPv4Iface(ax.tunnelIface).String() {
		if err := ax.setupInterface(); err != nil {
			return err
		}
	}

	// add routes and tunnels for all peer candidates without checking cache since it has not been built yet
	if firstTime {
		for _, peer := range cfg.Peers {
			ax.handlePeerRoute(peer)
			ax.handlePeerTunnel(peer)
		}
		return nil
	}

	// add routes and tunnels for the new peers only according to the cache diff
	for _, newPeer := range newPeers {
		if newPeer.Id != "" {
			// add routes for each peer candidate (unless the key matches the local nodes key)
			for _, peer := range cfg.Peers {
				if peer.PublicKey == newPeer.PublicKey && newPeer.PublicKey != ax.wireguardPubKey {
					ax.handlePeerRoute(peer)
					ax.handlePeerTunnel(peer)
				}
			}
		}
	}

	ax.logger.Infof("Peer setup complete")
	return nil
}
