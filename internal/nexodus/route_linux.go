//go:build linux

package nexodus

import "github.com/nexodus-io/nexodus/internal/models"

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (ax *Nexodus) handlePeerRoute(wgPeerConfig wgPeerConfig) {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		routeExists, err := RouteExists(allowedIP)
		if err != nil {
			ax.logger.Warnf("%v", err)
		}
		if !routeExists {
			if err := AddRoute(allowedIP, ax.tunnelIface); err != nil {
				ax.logger.Errorf("route add failed: %v", err)
			}
		}
	}
}

// handlePeerRoute when a peer is this handles route deletion
func (ax *Nexodus) handlePeerRouteDelete(dev string, wgPeerConfig models.Device) {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		routeExists, err := RouteExists(allowedIP)
		if !routeExists {
			continue
		}
		if err != nil {
			ax.logger.Debug(err)
		}
		if routeExists {
			if err := DeleteRoute(allowedIP, dev); err != nil {
				ax.logger.Debug(err)
			}
		}
	}
}
