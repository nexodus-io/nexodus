package nexodus

import (
	"github.com/nexodus-io/nexodus/internal/api/public"
)

const (
	persistentKeepalive = "20"
)

func (ax *Nexodus) DeployWireguardConfig(updatedPeers map[string]public.ModelsDevice) error {
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
	for _, updatedPeer := range updatedPeers {
		if updatedPeer.Id == "" {
			continue
		}
		// add routes for each peer candidate (unless the key matches the local nodes key)
		for _, peer := range cfg.Peers {
			if peer.PublicKey == updatedPeer.PublicKey && updatedPeer.PublicKey != ax.wireguardPubKey {
				ax.handlePeerRoute(peer)
				ax.handlePeerTunnel(peer)
			}
		}
	}

	ax.logger.Debug("Peer setup complete")
	return nil
}
