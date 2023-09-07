//go:build windows

package nexodus

import (
	"fmt"
	"net"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/util"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (nx *Nexodus) handlePeerRouteOS(wgPeerConfig wgPeerConfig) error {
	// If child prefix split the two prefixes (host /32) and child prefix
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !nx.ipv6Supported {
			continue
		}
		routeExists, err := RouteExistsOS(allowedIP)
		if err != nil {
			nx.logger.Debugf("failed to check if route exists: %v", err)
		}

		if util.IsIPv4Prefix(allowedIP) {
			if routeExists {
				if err := DeleteRoute(allowedIP, wgIface); err != nil {
					nx.logger.Debug(err)
				}
			}
			if err := AddRoute(allowedIP, wgIface); err != nil {
				nx.logger.Debug(err)
			}
		}

		if util.IsIPv6Prefix(allowedIP) {
			if routeExists {
				if err := DeleteRouteV6(allowedIP, wgIface); err != nil {
					nx.logger.Debug(err)
				}
			}
			if err := AddRouteV6(allowedIP, wgIface); err != nil {
				nx.logger.Debug(err)
			}
		}
	}

	return nil
}

// handlePeerRoute when a peer is this handles route deletion
func (nx *Nexodus) handlePeerRouteDeleteOS(dev string, wgPeerConfig public.ModelsDevice) {
	// TODO: Windoze route lookups
	for _, allowedIP := range wgPeerConfig.AllowedIps {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !nx.ipv6Supported {
			continue
		}
		if err := DeleteRoute(allowedIP, dev); err != nil {
			nx.logger.Debug(err)
		}
	}
}

func findInterfaceForIPRoute(ipRoute string) (*net.Interface, error) {
	ip, _, err := net.ParseCIDR(ipRoute)
	if err != nil {
		return nil, fmt.Errorf("invalid IP address or CIDR")
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range interfaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if v.IP.IsGlobalUnicast() && v.Contains(ip) {
					return &i, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no matching interface found")
}

// AddRoute adds a windows route to the specified interface
func AddRoute(prefix, dev string) error {
	// netsh interface ip add route [prefix] [interface|*]
	_, err := RunCommand("netsh", "interface", "ipv4", "add", "route", prefix, dev)
	if err != nil {
		return fmt.Errorf("no windows route added: %w", err)
	}

	return nil
}

// DeleteRoute deletes a windows route
func DeleteRoute(prefix, dev string) error {
	// netsh interface ip delete route [prefix] [interface|*]
	_, err := RunCommand("netsh", "interface", "ipv4", "del", "route", prefix, dev)
	if err != nil {
		return fmt.Errorf("no route deleted: %w", err)
	}

	return nil
}

// AddRouteV6 adds a route to the specified interface using netsh in Windows
func AddRouteV6(prefix, dev string) error {
	// netsh interface ipv6 add route [prefix] [interface] [gateway] [metric]
	_, err := RunCommand("netsh", "interface", "ipv6", "add", "route", prefix, dev, "::", "metric=256")
	if err != nil {
		return fmt.Errorf("v6 route add failed: %w", err)
	}

	return nil
}

// DeleteRouteV6 deletes a route from the specified interface using netsh in Windows
func DeleteRouteV6(prefix, dev string) error {
	// netsh interface ipv6 delete route [prefix] [interface]
	_, err := RunCommand("netsh", "interface", "ipv6", "delete", "route", prefix, dev)
	if err != nil {
		return fmt.Errorf("no route deleted: %w", err)
	}

	return nil
}
