//go:build windows

package apex

import (
	"fmt"
	"net"

	"go.uber.org/zap"
)

// routeExists currently only used for windows build purposes
func routeExists(s string) (bool, error) {
	return false, nil
}

// discoverLinuxAddress only used for windows build purposes
func discoverLinuxAddress(logger *zap.SugaredLogger, family int) (net.IP, error) {
	return nil, nil
}

func linkExists(wgIface string) bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Print(fmt.Errorf("localAddresses: %+v\n", err.Error()))
		return false
	}
	for _, iface := range ifaces {
		if iface.Name == wgIface {
			return true
		}
	}
	return false
}

// delLink only used for windows build purposes
func delLink(wgIface string) (err error) {
	return nil
}
