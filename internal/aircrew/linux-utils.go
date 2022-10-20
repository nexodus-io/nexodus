//go:build linux

package aircrew

import (
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// routeExists checks the netlink routes for the desitination prefix
func routeExists(prefix string) bool {
	destNet, err := ParseIPNet(prefix)
	if err != nil {
		log.Errorf("Failed to parse a valid network address from %s: %v", prefix, err)
	}
	destRoute := &netlink.Route{Dst: destNet}
	family := netlink.FAMILY_V6
	if destNet.IP.To4() != nil {
		family = netlink.FAMILY_V4
	}
	match, err := netlink.RouteListFiltered(family, destRoute, netlink.RT_FILTER_DST)
	if err != nil {
		log.Errorf("error retreiving netlink routes: %v", err)
		return true
	}
	if len(match) > 0 {
		return true
	}
	return false

}
