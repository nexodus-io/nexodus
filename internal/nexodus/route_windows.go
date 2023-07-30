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
	for _, allowedIP := range wgPeerConfig.AllowedIPs {
		// if the host does not support v6, skip adding the route
		if util.IsIPv6Prefix(allowedIP) && !nx.ipv6Supported {
			continue
		}
		if err := AddRoute(allowedIP, nx.tunnelIface); err != nil {
			nx.logger.Errorf("route add failed: %v", err)
			return err
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
