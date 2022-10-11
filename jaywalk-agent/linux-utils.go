package main

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// deleteIface checks to see if  is an interface exists and deletes it
func linkExists(ifaceName string) bool {
	if _, err := netlink.LinkByName(ifaceName); err != nil {
		return false
	}
	return true
}

// addLink adds a netlink interface
func addLink(ifaceName, linkType string, mtu int) error {
	wgLink := &netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: ifaceName,
			MTU:  mtu,
		},
		LinkType: linkType,
	}
	if err := netlink.LinkAdd(wgLink); err != nil {
		return fmt.Errorf("failed to create the netlink interface %s %v", ifaceName, err)
	}
	return nil
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

// setIP sets the IP address of a specified netlink interface
func setIP(ifaceName, ip string) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("failed to find the specified interface %s: %v", ifaceName, err)
	}
	linkaddr, err := netlink.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("failed to parse the interface ip address %s: %v", ip, err)
	}
	if err := netlink.AddrAdd(link, linkaddr); err != nil {
		return fmt.Errorf("failed to set ip address of the interface %s: %v", ifaceName, err)
	}
	return nil
}

// ifaceUP brings a specified netlink interface up
func ifaceUP(ifaceName string) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("failed to find the specified interface %s: %v", ifaceName, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to enable the specified interface %s: %v", ifaceName, err)
	}
	return nil
}

// routeAdd adds a netlink route, the gateway parameter is optional
func routeAdd(devName, gateway, dstCidr string) error {
	link, err := netlink.LinkByName(devName)
	if err != nil {
		return fmt.Errorf("failed to get the source interface to add a route %v", err)
	}
	route := netlink.Route{LinkIndex: link.Attrs().Index}
	if dstCidr == "" {
		return fmt.Errorf("destination network cannot be empty to add a route")
	}
	dstAddr, err := netlink.ParseAddr(dstCidr)
	if err != nil {
		return fmt.Errorf("failed to parse the route destination address %s: %v", dstCidr, err)
	}
	route.Dst = dstAddr.IPNet
	if gateway != "" {
		gatewayIP := net.ParseIP(gateway)
		route.Gw = gatewayIP
	}
	err = netlink.RouteAdd(&route)
	if err != nil {
		return fmt.Errorf("failed to add route %v", err)
	}
	return nil
}

// deleteIface checks to see if  is an interface exists and deletes it
func deleteIface(ifaceName string) error {
	if _, err := netlink.LinkByName(ifaceName); err == nil {
		log.Errorf("deleting existing interface %s", ifaceName)
		if err = delLink(ifaceName); err != nil {
			return fmt.Errorf("failed to delete existing interface %s: %v", ifaceName, err)
		}
	}
	return nil
}
