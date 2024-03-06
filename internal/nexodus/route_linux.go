//go:build linux

package nexodus

import (
	"fmt"
	"net"

	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/vishvananda/netlink"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (nx *Nexodus) handlePeerRouteOS(wgPeerConfig wgPeerConfig) error {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		// if the peer is advertising a default route, append it as an exit origin node, but don't add the route
		if util.IsDefaultIPv4Route(allowedIP) || util.IsDefaultIPv6Route(allowedIP) {
			nx.updateExitNodeOrigins(wgPeerConfig)
			continue
		}

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
func (nx *Nexodus) handlePeerRouteDeleteOS(dev string, wgPeerConfig client.ModelsDevice) {
	for _, allowedIP := range wgPeerConfig.AllowedIps {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !nx.ipv6Supported {
			continue
		}
		routeExists, err := nx.RouteExists(allowedIP)
		if !routeExists {
			continue
		}
		if err != nil {
			nx.logger.Debug(err)
		}
		if routeExists {
			if err := DeleteRoute(allowedIP, dev); err != nil {
				nx.logger.Debug(err)
			}
		}
	}
}

func findInterfaceForIPRoute(ip string) (*net.Interface, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP address")
	}

	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if route.Dst != nil && route.Dst.Contains(parsedIP) {
			link, err := netlink.LinkByIndex(route.LinkIndex)
			if err != nil {
				return nil, err
			}

			interfaceDetails, err := net.InterfaceByName(link.Attrs().Name)
			if err != nil {
				return nil, err
			}

			return interfaceDetails, nil
		}
	}

	return nil, fmt.Errorf("no matching interface found")
}
