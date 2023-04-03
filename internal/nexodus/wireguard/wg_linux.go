//go:build linux

package wireguard

import (
	"fmt"
	"strconv"

	"github.com/vishvananda/netlink"
)

func (wg *WireGuard) setupInterface() error {
	if wg.UserspaceMode {
		return wg.setupInterfaceUS()
	}
	return wg.setupInterfaceOS(wg.Relay)
}

// setupLinuxInterface TODO replace with netlink calls
// this is called if this is the first run or if the local node
// address got assigned a new address by the controller
func (wg *WireGuard) setupInterfaceOS(relay bool) error {

	logger := wg.Logger
	// delete the wireguard ip link interface if it exists
	if wg.ifaceExists() {
		_, err := runCommand("ip", "link", "del", wg.TunnelIface)
		if err != nil {
			logger.Debugf("failed to delete the ip link interface: %v\n", err)
		}
	}
	// create the wireguard ip link interface
	_, err := runCommand("ip", "link", "add", wg.TunnelIface, "type", "wireguard")
	if err != nil {
		logger.Errorf("failed to create the ip link interface: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}
	// start the wireguard listener on a well-known port if it is the hub-router as all
	// nodes need to be able to reach this node for state distribution if hole punching.
	if relay {
		_, err = runCommand("wg", "set", wg.TunnelIface, "listen-port", strconv.Itoa(WgDefaultPort), "private-key", linuxPrivateKeyFile)
		if err != nil {
			logger.Errorf("failed to start the wireguard listener: %v\n", err)
			return fmt.Errorf("%w", interfaceErr)
		}
	} else {
		// start the wireguard listener
		_, err = runCommand("wg", "set", wg.TunnelIface, "listen-port", strconv.Itoa(wg.ListenPort), "private-key", linuxPrivateKeyFile)
		if err != nil {
			logger.Errorf("failed to start the wireguard listener: %v\n", err)
			return fmt.Errorf("%w", interfaceErr)
		}
	}
	// give the wg interface an address
	_, err = runCommand("ip", "address", "add", wg.WgLocalAddress, "dev", wg.TunnelIface)
	if err != nil {
		logger.Debugf("failed to assign an address to the local linux interface, attempting to flush the iface: %v\n", err)
		wgIP := wg.getIPv4Iface()
		_, err = runCommand("ip", "address", "del", wgIP.To4().String(), "dev", wg.TunnelIface)
		if err != nil {
			logger.Errorf("failed to assign an address to the local linux interface: %v\n", err)
		}
		_, err = runCommand("ip", "address", "add", wg.TunnelIface, "dev", wg.TunnelIface)
		if err != nil {
			logger.Errorf("failed to assign an address to the local linux interface: %v\n", err)
			return fmt.Errorf("%w", interfaceErr)
		}
	}
	// bring the wg0 interface up
	_, err = runCommand("ip", "link", "set", wg.TunnelIface, "up")
	if err != nil {
		logger.Errorf("failed to bring up the wg interface: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}

	return nil
}

func (wg *WireGuard) RemoveExistingInterface() {

	if link, err := netlink.LinkByName(wg.TunnelIface); err == nil {
		if err = netlink.LinkDel(link); err != nil {
			wg.Logger.Debugf("failed to delete netlink interface %s: %v", wg.TunnelIface, err)
		}
	}
}
