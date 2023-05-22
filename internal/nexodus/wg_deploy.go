package nexodus

import (
	"errors"
	"runtime"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

const (
	persistentKeepalive = "20"
)

var (
	securityGroupErr      = errors.New("nftables setup error")
	securityGroupNotFound = errors.New("404 Not Found")
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

	if runtime.GOOS == Linux.String() {
		ax.logger.Debugf("Security group: %+v", ax.securityGroup)
		if err := ax.processSecurityGroupRules(); err != nil {
			return err
		}
	}

	// add routes and tunnels for the new peers only according to the cache diff
	for _, updatedPeer := range updatedPeers {
		if updatedPeer.Id == "" {
			continue
		}
		// add routes for each peer candidate (unless the key matches the local nodes key)
		peer, ok := cfg.Peers[updatedPeer.PublicKey]
		if !ok || peer.PublicKey == ax.wireguardPubKey {
			continue
		}
		ax.handlePeerRoute(peer)
		ax.handlePeerTunnel(peer)
	}

	ax.logger.Debug("Peer setup complete")
	return nil
}
