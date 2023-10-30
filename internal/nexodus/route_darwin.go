//go:build darwin

package nexodus

import (
	"fmt"
	"net"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/util"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (nx *Nexodus) handlePeerRouteOS(wgPeerConfig wgPeerConfig) error {
	// Darwin maps to a utunX address which needs to be discovered (currently hardcoded to utun8)
	devName, err := getInterfaceByIP(net.ParseIP(nx.TunnelIP))
	if err != nil {
		nx.logger.Errorf("failed to find the darwin interface with the address [ %s ] %v", nx.TunnelIP, err)
		return err
	}
	// If child prefix split the two prefixes (host /32) and child prefix
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
		routeExists, err := RouteExistsOS(allowedIP)
		if err != nil {
			nx.logger.Debugf("failed to check if route exists: %v", err)
		}

		if util.IsIPv4Prefix(allowedIP) {
			if routeExists {
				if err := DeleteRoute(allowedIP, devName); err != nil {
					nx.logger.Debugf("no route deleted: %v", err)
				}
			}

			if err := AddRoute(allowedIP, devName); err != nil {
				nx.logger.Errorf("%v", err)
				return err
			}
		}

		if util.IsIPv6Prefix(allowedIP) {
			if routeExists {
				if err := DeleteRouteV6(allowedIP, devName); err != nil {
					nx.logger.Debugf("no route deleted: %v", err)
				}
			}

			if err := AddRouteV6(allowedIP, devName); err != nil {
				nx.logger.Errorf("%v", err)
				return err
			}
		}
	}

	return nil
}

// handlePeerRoute when a peer is this handles route deletion
func (nx *Nexodus) handlePeerRouteDeleteOS(dev string, wgPeerConfig public.ModelsDevice) {
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
