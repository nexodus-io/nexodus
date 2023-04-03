//go:build linux

package wireguard

import (
	"fmt"
	"net"

	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/vishvananda/netlink"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (wg *WireGuard) handlePeerRouteOS(wgPeerConfig WgPeerConfig) {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		routeExists, err := wg.routeExists(allowedIP)
		if err != nil {
			wg.Logger.Warnf("%v", err)
		}
		if !routeExists {
			if err := addRoute(allowedIP, wg.TunnelIface); err != nil {
				wg.Logger.Errorf("route add failed: %v", err)
			}
		}
	}
}

// handlePeerRoute when a peer is this handles route deletion
func (wg *WireGuard) handlePeerRouteDeleteOS(dev string, wgPeerConfig models.Device) {
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		routeExists, err := wg.routeExists(allowedIP)
		if !routeExists {
			continue
		}
		if err != nil {
			wg.Logger.Debug(err)
		}
		if routeExists {
			if err := deleteRoute(allowedIP, dev); err != nil {
				wg.Logger.Debug(err)
			}
		}
	}
}

func addRoute(prefix, dev string) error {
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return fmt.Errorf("failed to lookup netlink device %s: %w", dev, err)
	}

	destNet, err := parseIPNet(prefix)
	if err != nil {
		return fmt.Errorf("failed to parse a valid network address from %s: %w", prefix, err)
	}

	return route(destNet, link)
}

// route adds a netlink route pointing to the linux device
func route(ipNet *net.IPNet, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       ipNet,
	})
}

// routeExistsOS checks netlink routes for the destination prefix
func routeExistsOS(prefix string) (bool, error) {
	destNet, err := parseIPNet(prefix)
	if err != nil {
		return false, fmt.Errorf("failed to parse a valid network address from %s: %w", prefix, err)
	}

	destRoute := &netlink.Route{Dst: destNet}
	family := netlink.FAMILY_V6
	if destNet.IP.To4() != nil {
		family = netlink.FAMILY_V4
	}

	match, err := netlink.RouteListFiltered(family, destRoute, netlink.RT_FILTER_DST)
	if err != nil {
		return true, fmt.Errorf("error retrieving netlink routes: %w", err)
	}

	if len(match) > 0 {
		return true, nil
	}

	return false, nil
}

// deleteRoute deletes a netlink route
func deleteRoute(prefix, dev string) error {
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return fmt.Errorf("unable to lookup interface %s", dev)
	}

	ipNet, err := parseIPNet(prefix)
	if err != nil {
		return err
	}
	routeSpec := netlink.Route{
		Dst:       ipNet,
		LinkIndex: link.Attrs().Index,
	}

	return netlink.RouteDel(&routeSpec)
}
