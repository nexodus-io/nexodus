package apex

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
)

const (
	// wg keepalives are disabled and managed by the agent
	persistentKeepalive    = "0"
	persistentHubKeepalive = "0"
)

func (ax *Apex) DeployWireguardConfig(newPeers []models.Peer, firstTime bool) error {
	cfg := &wgConfig{
		Interface: ax.wgConfig.Interface,
		Peers:     ax.wgConfig.Peers,
	}

	switch ax.os {
	case Linux.String():
		// re-initialize the wireguard interface if it does not match it's assigned node address
		if ax.wgLocalAddress != getIPv4Iface(wgIface).String() {
			ax.setupLinuxInterface(ax.logger)
		}

	case Darwin.String():
		// re-initialize the wireguard interface if it does not match it's assigned node address
		if ax.wgLocalAddress != getIPv4Iface(darwinIface).String() {
			if err := setupDarwinIface(ax.logger, ax.wgLocalAddress); err != nil {
				ax.logger.Errorf("%v", err)
			}
		}

	case Windows.String():
		ax.logger.Infof("windows is temporarily unsupported")
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
		if newPeer.ID != uuid.Nil {
			device, err := ax.client.GetDevice(newPeer.DeviceID)
			if err != nil {
				return fmt.Errorf("unable to get device %s: %s", newPeer.DeviceID, err)
			}
			// add routes for each peer candidate (unless the key matches the local nodes key)
			for _, peer := range cfg.Peers {
				if peer.PublicKey == device.PublicKey && device.PublicKey != ax.wireguardPubKey {
					ax.handlePeerRoute(peer)
					ax.handlePeerTunnel(peer)
				}
			}
		}
	}

	ax.logger.Infof("Peer setup complete")
	return nil
}
