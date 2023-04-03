//go:build linux

package nexodus

import (
	"fmt"
	"net"
	"net/url"

	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func discoverLinuxAddress(logger *zap.SugaredLogger, family int) (net.IP, error) {
	iface, _, err := getDefaultGatewayIface(logger, family)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	ips, err := getNetworkInterfaceIPs(iface)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("%w", err)
	}

	return ips[0].IP, nil
}

// getNetworkInterfaceIPs returns the IP addresses for the network interface
func getNetworkInterfaceIPs(iface string) ([]*net.IPNet, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup link %s: %w", iface, err)
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list addresses for %q: %w", iface, err)
	}

	var ips []*net.IPNet
	for _, addr := range addrs {
		if addr.IP.IsLinkLocalUnicast() {
			continue
		}
		if (addr.Flags & (unix.IFA_F_SECONDARY | unix.IFA_F_DEPRECATED)) != 0 {
			continue
		}
		ips = append(ips, addr.IPNet)
	}

	return ips, nil
}

// getDefaultGatewayIface returns the interface that contains the default gateway
func getDefaultGatewayIface(logger *zap.SugaredLogger, family int) (string, net.IP, error) {
	filter := &netlink.Route{Dst: nil}
	routes, err := netlink.RouteListFiltered(family, filter, netlink.RT_FILTER_DST)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get routing table in node: %w", err)
	}
	// use the first valid default gateway
	for _, r := range routes {
		if len(r.MultiPath) == 0 {
			ifaceLink, err := netlink.LinkByIndex(r.LinkIndex)
			if err != nil {
				logger.Warnf("Failed to get interface link for route %v : %v", r, err)
				continue
			}
			if r.Gw == nil {
				logger.Warnf("Failed to get gateway for route %v : %v", r, err)
				continue
			}
			logger.Infof("Found default gateway interface %s %s", ifaceLink.Attrs().Name, r.Gw.String())
			return ifaceLink.Attrs().Name, r.Gw, nil
		}
		for _, nh := range r.MultiPath {
			intfLink, err := netlink.LinkByIndex(nh.LinkIndex)
			if err != nil {
				logger.Warnf("Failed to get interface link for route %v : %v", nh, err)
				continue
			}
			if nh.Gw == nil {
				logger.Warnf("Failed to get gateway for multipath route %v : %v", nh, err)
				continue
			}
			logger.Infof("Found default gateway interface %s %s", intfLink.Attrs().Name, nh.Gw.String())
			return intfLink.Attrs().Name, nh.Gw, nil
		}
	}

	return "", net.IP{}, fmt.Errorf("failed to get default gateway interface")
}

func findLocalIP(logger *zap.SugaredLogger, controllerURL *url.URL) (string, error) {

	// Linux network discovery
	linuxIP, err := discoverLinuxAddress(logger, 4)
	if err != nil {
		return "", err
	}
	return linuxIP.String(), nil
}
