//go:build darwin

package apex

import (
	"net"

	"github.com/vishvananda/netlink"
)

// routeExists currently only used for darwin build purposes
func routeExists(s string) bool {
	return false
}

// discoverLinuxAddress only used for darwin build purposes
func discoverLinuxAddress(family int) (net.IP, error) {
	return nil, nil
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
