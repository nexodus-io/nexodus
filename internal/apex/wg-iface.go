package apex

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// getInterfaceByIP will looks ip an interface by the IP provided
func getInterfaceByIP(ip net.IP) (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ifaceIP, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ifaceIP.Equal(ip) {
				return iface.Name, nil
			}
		}
	}

	return "", fmt.Errorf("no interface was found for the ip %s", ip)
}

func setupDarwinIface(logger *zap.SugaredLogger, localAddress, dev string) error {
	if ifaceExists(logger, dev) {
		deleteDarwinIface(logger, dev)
	}

	_, err := RunCommand("wireguard-go", dev)
	if err != nil {
		logger.Errorf("failed to create the %s interface: %v\n", dev, err)
	}

	_, err = RunCommand("ifconfig", dev, "inet", localAddress, localAddress, "alias")
	if err != nil {
		logger.Errorf("failed to assign an address to the local osx interface: %v\n", err)
	}

	_, err = RunCommand("ifconfig", dev, "up")
	if err != nil {
		logger.Errorf("failed to bring up the %s interface: %v\n", dev, err)
	}

	_, err = RunCommand("wg", "set", dev, "private-key", darwinPrivateKeyFile)
	if err != nil {
		logger.Errorf("failed to start the wireguard listener: %v\n", err)
	}

	return nil
}

// setupLinuxInterface TODO replace with netlink calls
// this is called if this is the first run or if the local node
// address got assigned a new address by the controller
func (ax *Apex) setupLinuxInterface(logger *zap.SugaredLogger) {
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
	}
	// start the wireguard listener on a well-known port if it is the hub-router as all
	// nodes need to be able to reach this node for state distribution if hole punching.
	if ax.hubRouter {
		_, err = RunCommand("wg", "set", ax.tunnelIface, "listen-port", strconv.Itoa(WgDefaultPort), "private-key", linuxPrivateKeyFile)
		if err != nil {
			logger.Errorf("failed to start the wireguard listener: %v\n", err)
		}
	} else {
		// start the wireguard listener
		_, err = RunCommand("wg", "set", ax.tunnelIface, "listen-port", strconv.Itoa(ax.listenPort), "private-key", linuxPrivateKeyFile)
		if err != nil {
			logger.Errorf("failed to start the wireguard listener: %v\n", err)
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
		}
	}
	// bring the wg0 interface up
	_, err = RunCommand("ip", "link", "set", ax.tunnelIface, "up")
	if err != nil {
		logger.Errorf("failed to bring up the wg interface: %v\n", err)
	}
}

func setupWindowsIface(logger *zap.SugaredLogger, localAddress, privateKey, dev string) error {
	if err := buildWindowsWireguardIfaceConf(privateKey, localAddress); err != nil {
		return fmt.Errorf("failed to create the windows wireguard wg0 interface file: %w", err)
	}

	var wgOut string
	var err error
	if ifaceExists(logger, dev) {
		wgOut, err = RunCommand("wireguard.exe", "/uninstalltunnelservice", dev)
		if err != nil {
			logger.Debugf("failed to down the wireguard interface (this is generally ok): %w", err)
		}
		if ifaceExists(logger, dev) {
			logger.Debugf("existing windows iface %s has not been torn down yet, pausing for 1 second", dev)
			time.Sleep(time.Second * 1)
		}
	}
	logger.Debugf("stopped windows tunnel svc:%v\n", wgOut)
	// sleep for one second to give the wg async exe time to tear down any existing wg0 configuration
	wgOut, err = RunCommand("wireguard.exe", "/installtunnelservice", windowsWgConfigFile)
	if err != nil {
		return fmt.Errorf("failed to start the wireguard interface: %w", err)
	}
	logger.Debugf("started windows tunnel svc: %v\n", wgOut)
	// ensure the link is created before adding peers
	if !ifaceExists(logger, dev) {
		logger.Debugf("windows iface %s has not been created yet, pausing for 1 second", dev)
		time.Sleep(time.Second * 1)
	}
	// fatal out if the interface is not created
	if !ifaceExists(logger, dev) {
		return fmt.Errorf("failed to create the windows wireguard interface: %w", err)
	}

	return nil
}

// deleteDarwinIface delete the darwin userspace wireguard interface
func deleteDarwinIface(logger *zap.SugaredLogger, dev string) {
	tunSock := fmt.Sprintf("/var/run/wireguard/%s.sock", dev)
	_, err := RunCommand("rm", "-f", tunSock)
	if err != nil {
		logger.Debugf("failed to delete darwin interface: %v", err)
	}
	// /var/run/wireguard/wg0.name doesnt currently exist since utun8 isnt mapped to wg0 (fails silently)
	wgName := fmt.Sprintf("/var/run/wireguard/%s.name", dev)
	_, err = RunCommand("rm", "-f", wgName)
	if err != nil {
		logger.Debugf("failed to delete darwin interface: %v", err)
	}
}

// ifaceExists returns true if the input matches a net interface
func ifaceExists(logger *zap.SugaredLogger, iface string) bool {
	_, err := net.InterfaceByName(iface)
	if err != nil {
		logger.Debugf("existing link not found: %s", iface)
		return false
	}

	return true
}

// getIPv4Iface get the IP of the specified net interface
func getIPv4Iface(ifname string) net.IP {
	interfaces, _ := net.Interfaces()
	for _, inter := range interfaces {
		if inter.Name != ifname {
			continue
		}
		addrs, err := inter.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch ip := addr.(type) {
			case *net.IPNet:
				if ip.IP.DefaultMask() != nil {
					return ip.IP
				}
			}
		}
	}

	return nil
}

// enableForwardingIPv4 for linux nodes that are hub bouncers
func enableForwardingIPv4(logger *zap.SugaredLogger) error {
	cmdOut, err := RunCommand("sysctl", "-w", "net.ipv4.ip_forward=1")
	if err != nil {
		return fmt.Errorf("failed to enable IP Forwarding for this hub-router: %w", err)
	}
	logger.Debugf("%v", cmdOut)
	return nil
}
