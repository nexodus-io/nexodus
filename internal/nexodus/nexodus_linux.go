//go:build linux

package nexodus

import (
	"fmt"
	"strconv"
)

// setupLinuxInterface TODO replace with netlink calls
// this is called if this is the first run or if the local node
// address got assigned a new address by the controller
func (ax *Nexodus) setupInterfaceOS() error {

	logger := ax.logger
	// delete the wireguard ip link interface if it exists
	if ifaceExists(logger, ax.tunnelIface) {
		_, err := RunCommand("ip", "link", "del", ax.tunnelIface)
		if err != nil {
			logger.Debugf("failed to delete the ip link interface: %v\n", err)
		}
	}
	// create the wireguard ip link interface
	_, err := RunCommand("ip", "link", "add", ax.tunnelIface, "type", "wireguard")
	if err != nil {
		logger.Errorf("failed to create the ip link interface: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}
	// start the wireguard listener on a well-known port if it is the hub-router as all
	// nodes need to be able to reach this node for state distribution if hole punching.
	if ax.relay {
		_, err = RunCommand("wg", "set", ax.tunnelIface, "listen-port", strconv.Itoa(WgDefaultPort), "private-key", linuxPrivateKeyFile)
		if err != nil {
			logger.Errorf("failed to start the wireguard listener: %v\n", err)
			return fmt.Errorf("%w", interfaceErr)
		}
	} else {
		// start the wireguard listener
		_, err = RunCommand("wg", "set", ax.tunnelIface, "listen-port", strconv.Itoa(ax.listenPort), "private-key", linuxPrivateKeyFile)
		if err != nil {
			logger.Errorf("failed to start the wireguard listener: %v\n", err)
			return fmt.Errorf("%w", interfaceErr)
		}
	}
	// give the wg interface an address
	_, err = RunCommand("ip", "address", "add", ax.wgLocalAddress, "dev", ax.tunnelIface)
	if err != nil {
		logger.Debugf("failed to assign an address to the local linux interface, attempting to flush the iface: %v\n", err)
		wgIP := getIPv4Iface(ax.tunnelIface)
		_, err = RunCommand("ip", "address", "del", wgIP.To4().String(), "dev", ax.tunnelIface)
		if err != nil {
			logger.Errorf("failed to assign an address to the local linux interface: %v\n", err)
		}
		_, err = RunCommand("ip", "address", "add", ax.wgLocalAddress, "dev", ax.tunnelIface)
		if err != nil {
			logger.Errorf("failed to assign an address to the local linux interface: %v\n", err)
			return fmt.Errorf("%w", interfaceErr)
		}
	}
	// bring the wg0 interface up
	_, err = RunCommand("ip", "link", "set", ax.tunnelIface, "up")
	if err != nil {
		logger.Errorf("failed to bring up the wg interface: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}

	return nil
}

func (ax *Nexodus) removeExistingInterface() {
	if linkExists(ax.tunnelIface) {
		if err := delLink(ax.tunnelIface); err != nil {
			// not a fatal error since if this is on startup it could be absent
			ax.logger.Debugf("failed to delete netlink interface %s: %v", ax.tunnelIface, err)
		}
	}
}

func (ax *Nexodus) findLocalIP() (string, error) {

	// Linux network discovery
	linuxIP, err := discoverLinuxAddress(ax.logger, 4)
	if err != nil {
		return "", err
	}
	return linuxIP.String(), nil
}
