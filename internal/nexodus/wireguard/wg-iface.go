package wireguard

import (
	"errors"
	"fmt"
	"net"

	"go.uber.org/zap"
)

var interfaceErr = errors.New("interface setup error")

// ifaceExists returns true if the input matches a net interface
func (wg *WireGuard) ifaceExists() bool {
	_, err := net.InterfaceByName(wg.TunnelIface)
	if err != nil {
		wg.Logger.Debugf("existing link not found: %s", wg.TunnelIface)
		return false
	}

	return true
}

// getIPv4Iface get the IP of the specified net interface
func (wg *WireGuard) getIPv4Iface() net.IP {
	if wg.UserspaceMode {
		return getIPv4IfaceUS(wg.UserspaceLastAddress)
	} else {
		return getIPv4IfaceOS(wg.TunnelIface)
	}
}

func getIPv4IfaceUS(ifname string) net.IP {
	return net.ParseIP(ifname)
}

func getIPv4IfaceOS(ifname string) net.IP {
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

// EnableForwardingIPv4 for linux nodes that are hub bouncers
func EnableForwardingIPv4(logger *zap.SugaredLogger) error {
	cmdOut, err := runCommand("sysctl", "-w", "net.ipv4.ip_forward=1")
	if err != nil {
		return fmt.Errorf("failed to enable IP Forwarding for this hub-router: %w", err)
	}
	logger.Debugf("%v", cmdOut)
	return nil
}
