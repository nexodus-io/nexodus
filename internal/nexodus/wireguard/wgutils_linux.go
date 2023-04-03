//go:build linux

package wireguard

import (
	"fmt"
	"net"
	"os/exec"

	"go.uber.org/zap"
)

// ParseIPNet return an IPNet from a string
func parseIPNet(s string) (*net.IPNet, error) {
	ip, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	return &net.IPNet{IP: ip, Mask: ipNet.Mask}, nil
}

func defaultTunnelDevOS() string {
	return wgIface
}

// binaryChecks validate the required binaries are available
func binaryChecks() error {
	// all OSs require the wg binary
	if _, err := exec.LookPath(wgBinary); err != nil {
		return fmt.Errorf("%s command not found, is wireguard installed?", wgBinary)
	}
	return nil
}

// checkOS and report error if the OS is not supported.
func checkOS(logger *zap.SugaredLogger) error {
	// ensure the linux wireguard directory exists
	if err := CreateDirectory(WgLinuxConfPath); err != nil {
		return fmt.Errorf("unable to create the wireguard config directory [%s]: %w", WgLinuxConfPath, err)
	}
	return nil
}
