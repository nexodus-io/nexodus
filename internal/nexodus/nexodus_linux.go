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

	if ax.TunnelIP == "" || ax.TunnelIpV6 == "" {
		return fmt.Errorf("Have not received local node address configuration from the service, returning for a retry")
	}

	// create the wireguard ip link interface
	_, err := RunCommand("ip", "link", "add", ax.tunnelIface, "type", "wireguard")
	if err != nil {
		logger.Errorf("failed to create the ip link interface: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}
	// start the wireguard listener on a well-known port if it is a discovery or relay node as all
	// nodes need to be able to reach those services for state distribution, hole punching and relay.
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

	// assign the wg interface a v6 address
	if ax.ipv6Supported {
		localAddressIPv6 := fmt.Sprintf("%s/%s", ax.TunnelIpV6, wgOrgIPv6PrefixLen)
		_, err = RunCommand("ip", "-6", "address", "add", localAddressIPv6, "dev", ax.tunnelIface)
		if err != nil {
			logger.Infof("failed to assign an IPv6 address to the local linux ipv6 interface, ensure v6 is supported: %v\n", err)
		}
	}

	// assign the wg interface a v4 address, delete the existing if one is present
	_, err = RunCommand("ip", "address", "add", ax.TunnelIP, "dev", ax.tunnelIface)
	if err != nil {
		logger.Debugf("failed to assign an address to the local linux interface, attempting to flush the iface: %v\n", err)
		wgIP := ax.getIPv4Iface(ax.tunnelIface)
		// TODO: this is likely legacy from a push model, should be ok to remove the deletes since the agent now deletes wg0 on startup
		_, err = RunCommand("ip", "address", "del", wgIP.To4().String(), "dev", ax.tunnelIface)
		if err != nil {
			logger.Errorf("failed to assign an IPv4 address to the local linux interface: %v\n", err)
		}
		_, err = RunCommand("ip", "address", "add", ax.TunnelIP, "dev", ax.tunnelIface)
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
