//go:build linux

package nexodus

import (
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/util"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (nx *Nexodus) handlePeerRouteOS(wgPeerConfig wgPeerConfig) error {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !nx.ipv6Supported {
			continue
		}
		routeExists, err := nx.RouteExists(allowedIP)
		if err != nil {
			nx.logger.Warnf("%v", err)
		}
		if !routeExists {
			if err := AddRoute(allowedIP, nx.tunnelIface); err != nil {
				nx.logger.Errorf("route add failed: %v", err)
				return err
			}
		}
	}
	return nil
}

// handlePeerRoute when a peer is this handles route deletion
func (ax *Nexodus) handlePeerRouteDeleteOS(dev string, wgPeerConfig public.ModelsDevice) {
	for _, allowedIP := range wgPeerConfig.AllowedIps {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !ax.ipv6Supported {
			continue
		}
		routeExists, err := ax.RouteExists(allowedIP)
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
