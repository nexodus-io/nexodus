//go:build windows

package nexodus

import "github.com/nexodus-io/nexodus/internal/models"

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (ax *Nexodus) handlePeerRoute(wgPeerConfig wgPeerConfig) {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		if err := AddRoute(allowedIP, ax.tunnelIface); err != nil {
			ax.logger.Debugf("route add failed: %v", err)
		}
	}
}

// handlePeerRoute when a peer is this handles route deletion
func (ax *Nexodus) handlePeerRouteDelete(dev string, wgPeerConfig models.Device) {
	// TODO: Windoze route lookups
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		if err := DeleteRoute(allowedIP, dev); err != nil {
			ax.logger.Debug(err)
		}
	}
}
