//go:build darwin

package wireguard

import (
	"fmt"
	"net"

	"github.com/nexodus-io/nexodus/internal/models"
)

// handlePeerRouteOS when a new configuration is deployed, delete/add the peer allowedIPs
func (wg *WireGuard) handlePeerRouteOS(wgPeerConfig WgPeerConfig) {
	// Darwin maps to a utunX address which needs to be discovered (currently hardcoded to utun8)
	devName, err := getInterfaceByIP(net.ParseIP(wg.WgLocalAddress))
	if err != nil {
		wg.Logger.Debugf("failed to find the darwin interface with the address [ %s ] %v", wg.WgLocalAddress, err)
	}
	// If child prefix split the two prefixes (host /32) and child prefix
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		_, err := runCommand("route", "-q", "-n", "delete", "-inet", allowedIP, "-interface", devName)
		if err != nil {
			wg.Logger.Debugf("no route deleted: %v", err)
		}
		if err := addRoute(allowedIP, devName); err != nil {
			wg.Logger.Debugf("%v", err)
		}
	}

}

func (wg *WireGuard) handlePeerRouteDeleteOS(dev string, wgPeerConfig models.Device) {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		if err := deleteRoute(allowedIP, dev); err != nil {
			wg.Logger.Debug(err)
		}
	}

}

// getInterfaceByIP looks up an interface by the IP provided
func getInterfaceByIP(ip net.IP) (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ifaceIP, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ifaceIP.Equal(ip) {
				return iface.Name, nil
			}
		}
	}

	return "", fmt.Errorf("no interface was found for the ip %s", ip)
}

// routeExistsOS currently only used for darwin build purposes
func routeExistsOS(s string) (bool, error) {
	return false, nil
}

// AddRoute adds a route to the specified interface
func addRoute(prefix, dev string) error {
	_, err := runCommand("route", "-q", "-n", "add", "-inet", prefix, "-interface", dev)
	if err != nil {
		return fmt.Errorf("route add failed: %w", err)
	}

	return nil
}

// deleteRoute deletes a darwin route
func deleteRoute(prefix, dev string) error {
	_, err := runCommand("route", "-q", "-n", "delete", "-inet", prefix, "-interface", dev)
	if err != nil {
		return fmt.Errorf("no route deleted: %w", err)
	}

	return nil
}
