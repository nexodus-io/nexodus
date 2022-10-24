//go:build linux

package apex

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// routeExists checks the netlink routes for the destination prefix
func routeExists(prefix string) bool {
	destNet, err := ParseIPNet(prefix)
	if err != nil {
		log.Errorf("failed to parse a valid network address from %s: %v", prefix, err)
	}
	destRoute := &netlink.Route{Dst: destNet}
	family := netlink.FAMILY_V6
	if destNet.IP.To4() != nil {
		family = netlink.FAMILY_V4
	}
	match, err := netlink.RouteListFiltered(family, destRoute, netlink.RT_FILTER_DST)
	if err != nil {
		log.Errorf("error retrieving netlink routes: %v", err)
		return true
	}
	if len(match) > 0 {
		return true
	}
	return false
}

func discoverLinuxAddress(family int) (net.IP, error) {
	iface, _, err := getDefaultGatewayIface(family)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}
	ips, err := getNetworkInterfaceIPs(iface)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("%v", err)
	}
	return ips[0].IP, nil
}

// getNetworkInterfaceIPs returns the IP addresses for the network interface
func getNetworkInterfaceIPs(iface string) ([]*net.IPNet, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup link %s: %v", iface, err)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list addresses for %q: %v", iface, err)
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
func getDefaultGatewayIface(family int) (string, net.IP, error) {
	filter := &netlink.Route{Dst: nil}
	routes, err := netlink.RouteListFiltered(family, filter, netlink.RT_FILTER_DST)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get routing table in node: %v", err)
	}
	// use the first valid default gateway
	for _, r := range routes {
		if len(r.MultiPath) == 0 {
			ifaceLink, err := netlink.LinkByIndex(r.LinkIndex)
			if err != nil {
				log.Warningf("Failed to get interface link for route %v : %v", r, err)
				continue
			}
			if r.Gw == nil {
				log.Warningf("Failed to get gateway for route %v : %v", r, err)
				continue
			}
			log.Infof("Found default gateway interface %s %s", ifaceLink.Attrs().Name, r.Gw.String())
			return ifaceLink.Attrs().Name, r.Gw, nil
		}
		for _, nh := range r.MultiPath {
			intfLink, err := netlink.LinkByIndex(nh.LinkIndex)
			if err != nil {
				log.Warningf("Failed to get interface link for route %v : %v", nh, err)
				continue
			}
			if nh.Gw == nil {
				log.Warningf("Failed to get gateway for multipath route %v : %v", nh, err)
				continue
			}
			log.Infof("Found default gateway interface %s %s", intfLink.Attrs().Name, nh.Gw.String())
			return intfLink.Attrs().Name, nh.Gw, nil
		}
	}
	return "", net.IP{}, fmt.Errorf("failed to get default gateway interface")
}

// deleteIface checks to see if  is an interface exists and deletes it
func linkExists(ifaceName string) bool {
	if _, err := netlink.LinkByName(ifaceName); err != nil {
		return false
	}
	return true
}

// delLink deletes the link and assumes it exists
func delLink(ifaceName string) error {
	if link, err := netlink.LinkByName(ifaceName); err == nil {
		if err = netlink.LinkDel(link); err != nil {
			return err
		}
	}
	return nil
}
