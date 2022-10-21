//go:build linux

package apex

import (
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
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
