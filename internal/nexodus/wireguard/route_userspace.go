package wireguard

import (
	"github.com/nexodus-io/nexodus/internal/models"
)

// All route management is a no-op in userspace mode.
// We have a network stack with a single device and a
// default route that uses that device. No other routes
// provide any value.
//
// Explicit routes only for the addresses we should be
// able to reach would be better, but route management
// isn't accessible through the netstack package in
// wireguard-go. It's possible in underlying gvisor
// code, but we can't get to it.

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (wg *WireGuard) handlePeerRouteUS(wgPeerConfig WgPeerConfig) {
	// no-op
}

// handlePeerRoute when a peer is this handles route deletion
func (wg *WireGuard) handlePeerRouteDeleteUS(dev string, wgPeerConfig models.Device) {
	// no-op
}

func routeExistsUS(prefix string) (bool, error) {
	// no-op
	return false, nil
}

func (wg *WireGuard) addRouteUS(prefix string) error {
	// no-op
	return nil
}
