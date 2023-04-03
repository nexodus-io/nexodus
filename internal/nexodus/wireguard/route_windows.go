//go:build windows

package wireguard

import (
	"fmt"

	"github.com/nexodus-io/nexodus/internal/models"
)

// handlePeerRouteOS when a new configuration is deployed, delete/add the peer allowedIPs
func (wg *WireGuard) handlePeerRouteOS(wgPeerConfig WgPeerConfig) {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		if err := addRoute(allowedIP, wg.TunnelIface); err != nil {
			wg.Logger.Debugf("route add failed: %v", err)
		}
	}
}

// handlePeerRouteDeleteOS when a peer is this handles route deletion
func (wg *WireGuard) handlePeerRouteDeleteOS(dev string, wgPeerConfig models.Device) {
	// TODO: Windoze route lookups
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		if err := deleteRoute(allowedIP, dev); err != nil {
			wg.Logger.Debug(err)
		}
	}
}

// routeExistsOS currently only used for windows build purposes
func routeExistsOS(s string) (bool, error) {
	return false, nil
}

// addRoute adds a windows route to the specified interface
func addRoute(prefix, dev string) error {
	// TODO: replace with powershell
	_, err := runCommand("netsh", "int", "ipv4", "add", "route", prefix, dev)
	if err != nil {
		return fmt.Errorf("no windows route added: %w", err)
	}

	return nil
}

// deleteRoute deletes a windows route
func deleteRoute(prefix, dev string) error {
	_, err := runCommand("netsh", "int", "ipv4", "del", "route", prefix, dev)
	if err != nil {
		return fmt.Errorf("no route deleted: %w", err)
	}

	return nil
}
