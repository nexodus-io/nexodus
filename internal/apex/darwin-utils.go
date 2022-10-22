//go:build darwin

package apex

import "net"

// routeExists currently only used for darwin build purposes
func routeExists(s string) bool {
	return false
}

// discoverLinuxAddress only used for darwin build purposes
func discoverLinuxAddress(family int) (net.IP, error) {
	return nil, nil
}
