package wireguard

import (
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
)

func (wg *WireGuard) DeployWireguardConfig(newPeers []models.Device, firstTime bool) error {
	if wg.WgLocalAddress != wg.getIPv4Iface().String() {
		if err := wg.setupInterface(); err != nil {
			return err
		}
	}

	// add routes and tunnels for all peer candidates without checking cache since it has not been built yet
	if firstTime {
		for _, peer := range wg.Peers {
			wg.handlePeerRoute(peer)
			wg.handlePeerTunnel(peer)
		}
		return nil
	}

	// add routes and tunnels for the new peers only according to the cache diff
	for _, newPeer := range newPeers {
		if newPeer.ID != uuid.Nil {
			// add routes for each peer candidate (unless the key matches the local nodes key)
			for _, peer := range wg.Peers {
				if peer.PublicKey == newPeer.PublicKey && newPeer.PublicKey != wg.WireguardPubKey {
					wg.handlePeerRoute(peer)
					wg.handlePeerTunnel(peer)
				}
			}
		}
	}

	wg.Logger.Infof("Peer setup complete")
	return nil
}
