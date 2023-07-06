//go:build windows

package nexodus

import (
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/util"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (ax *Nexodus) handlePeerRouteOS(wgPeerConfig wgPeerConfig) error {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !ax.ipv6Supported {
			continue
		}
		if err := AddRoute(allowedIP, ax.tunnelIface); err != nil {
			ax.logger.Errorf("route add failed: %v", err)
			return err
		}
	}
	return nil
}

// handlePeerRoute when a peer is this handles route deletion
func (ax *Nexodus) handlePeerRouteDeleteOS(dev string, wgPeerConfig public.ModelsDevice) {
	// TODO: Windoze route lookups
	for _, allowedIP := range wgPeerConfig.AllowedIps {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !ax.ipv6Supported {
			continue
		}
		if err := DeleteRoute(allowedIP, dev); err != nil {
			ax.logger.Debug(err)
		}
	}
}
