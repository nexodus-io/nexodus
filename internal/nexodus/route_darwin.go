//go:build darwin

package nexodus

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"net"

	"github.com/nexodus-io/nexodus/internal/util"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (ax *Nexodus) handlePeerRouteOS(wgPeerConfig wgPeerConfig) {
	// Darwin maps to a utunX address which needs to be discovered (currently hardcoded to utun8)
	devName, err := getInterfaceByIP(net.ParseIP(ax.TunnelIP))
	if err != nil {
		ax.logger.Debugf("failed to find the darwin interface with the address [ %s ] %v", ax.TunnelIP, err)
	}
	// If child prefix split the two prefixes (host /32) and child prefix
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !ax.ipv6Supported {
			continue
		}

		if util.IsIPv4Prefix(allowedIP) {
			_, err := RunCommand("route", "-q", "-n", "delete", "-inet", allowedIP, "-interface", devName)
			if err != nil {
				ax.logger.Debugf("no route deleted: %v", err)
			}
			if err := AddRoute(allowedIP, devName); err != nil {
				ax.logger.Debugf("%v", err)
			}
		}

		if util.IsIPv6Prefix(allowedIP) {
			_, err := RunCommand("route", "-q", "-n", "delete", "-inet6", allowedIP, "-interface", devName)
			if err != nil {
				ax.logger.Debugf("no route deleted: %v", err)
			}
			if err := AddRouteV6(allowedIP, devName); err != nil {
				ax.logger.Debugf("%v", err)
			}
		}
	}

}

// handlePeerRoute when a peer is this handles route deletion
func (ax *Nexodus) handlePeerRouteDeleteOS(dev string, wgPeerConfig public.ModelsDevice) {
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
