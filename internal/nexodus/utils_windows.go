//go:build windows

package nexodus

import (
	"fmt"
	"net"

	"go.uber.org/zap"
)

// RouteExistsOS currently only used for windows build purposes
func RouteExistsOS(s string) (bool, error) {
	return false, nil
}

// AddRoute adds a windows route to the specified interface
func AddRoute(prefix, dev string) error {
	// TODO: replace with powershell
	_, err := RunCommand("netsh", "int", "ipv4", "add", "route", prefix, dev)
	if err != nil {
		return fmt.Errorf("no windows route added: %w", err)
	}

	return nil
}

// discoverLinuxAddress only used for windows build purposes
func discoverLinuxAddress(logger *zap.SugaredLogger, family int) (net.IP, error) {
	return nil, nil
}

func linkExists(wgIface string) bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Print(fmt.Errorf("localAddresses: %w\n", err))
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

// DeleteRoute deletes a windows route
func DeleteRoute(prefix, dev string) error {
	_, err := RunCommand("netsh", "int", "ipv4", "del", "route", prefix, dev)
	if err != nil {
		return fmt.Errorf("no route deleted: %w", err)
	}

	return nil
}

func defaultTunnelDevOS() string {
	return wgIface
}

// binaryChecks validate the required binaries are available
func binaryChecks() error {
	if !IsCommandAvailable(wgWinBinary) {
		return fmt.Errorf("%s command not found, is wireguard installed?", wgWinBinary)
	}
	return nil
}

// Check OS and report error if the OS is not supported.
func checkOS(logger *zap.SugaredLogger) error {
	// ensure the windows wireguard directory exists
	if err := CreateDirectory(WgWindowsConfPath); err != nil {
		return fmt.Errorf("unable to create the wireguard config directory [%s]: %w", WgWindowsConfPath, err)
	}
	return nil
}
