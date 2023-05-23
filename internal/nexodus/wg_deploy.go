package nexodus

import (
	"github.com/nexodus-io/nexodus/internal/api/public"
)

const (
	persistentKeepalive = "20"
)

func (ax *Nexodus) DeployWireguardConfig(newPeers []public.ModelsDevice) error {
	cfg := &wgConfig{
		Interface: ax.wgConfig.Interface,
		Peers:     ax.wgConfig.Peers,
	}

	if ax.TunnelIP != ax.getIPv4Iface(ax.tunnelIface).String() {
		if err := ax.setupInterface(); err != nil {
			return err
		}
	}

	// add routes and tunnels for the new peers only according to the cache diff
	for _, newPeer := range newPeers {
		if newPeer.Id == "" {
			continue
		}
		// add routes for each peer candidate (unless the key matches the local nodes key)
		for _, peer := range cfg.Peers {
			if peer.PublicKey == newPeer.PublicKey && newPeer.PublicKey != ax.wireguardPubKey {
				ax.handlePeerRoute(peer)
				ax.handlePeerTunnel(peer)
			}
		}
	}

	ax.logger.Debug("Peer setup complete")
	return nil
}
