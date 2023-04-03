//go:build windows

package nexodus

import (
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (ax *Nexodus) handlePeerRouteOS(wgPeerConfig wgPeerConfig) {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !ax.ipv6Supported {
			continue
		}
		if err := AddRoute(allowedIP, ax.tunnelIface); err != nil {
			ax.logger.Debugf("route add failed: %v", err)
		}
	}
}

// handlePeerRoute when a peer is this handles route deletion
func (ax *Nexodus) handlePeerRouteDeleteOS(dev string, wgPeerConfig models.Device) {
	// TODO: Windoze route lookups
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !ax.ipv6Supported {
			continue
		}
		if err := DeleteRoute(allowedIP, dev); err != nil {
			ax.logger.Debug(err)
		}
	}
}
