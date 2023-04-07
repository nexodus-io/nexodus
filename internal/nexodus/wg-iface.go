package nexodus

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"go.uber.org/zap"
)

const (
	fwdFilePathV4 = "/proc/sys/net/ipv4/ip_forward"
	fwdFilePathV6 = "/proc/sys/net/ipv6/conf/all/forwarding"
)

var interfaceErr = errors.New("interface setup error")

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
func (ax *Nexodus) getIPv4Iface(ifname string) net.IP {
	if ax.userspaceMode {
		return ax.getIPv4IfaceUS(ifname)
	} else {
		return ax.getIPv4IfaceOS(ifname)
	}
}

func (ax *Nexodus) getIPv4IfaceUS(ifname string) net.IP {
	return net.ParseIP(ax.userspaceLastAddress)
}

func (ax *Nexodus) getIPv4IfaceOS(ifname string) net.IP {
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

// relayPrep prepare a node to be a relay, enable ip forwarding if not already done so and iptables rules for v4/v6
func (ax *Nexodus) relayPrep() error {
	ipv4FwdEnabled, err := isIPForwardingEnabled(fwdFilePathV4)
	if err != nil {
		return err
	}

	if !ipv4FwdEnabled {
		if err := enableForwardingIPv4(); err != nil {
			return err
		}
	}

	ipv6FwdEnabled, err := isIPForwardingEnabled(fwdFilePathV6)
	if err != nil {
		return err
	}

	if !ipv6FwdEnabled {
		if err := enableForwardingIPv6(); err != nil {
			return err
		}
	}

	if err := relayIpTables(ax.logger, ax.tunnelIface); err != nil {
		return err
	}

	return nil
}

// enableForwardingIPv4 for v4 linux nodes that are relay nodes
func enableForwardingIPv4() error {
	_, err := RunCommand("sysctl", "-w", "net.ipv4.ip_forward=1")
	if err != nil {
		return fmt.Errorf("failed to enable IPv4 Forwarding for this relay node: %w", err)
	}

	return nil
}

// enableForwardingIPv6 for v6 linux nodes that are relay nodes
func enableForwardingIPv6() error {
	_, err := RunCommand("sysctl", "-w", "net.ipv6.conf.all.forwarding=1")
	if err != nil {
		return fmt.Errorf("failed to enable IPv6 Forwarding for this relay node: %w", err)
	}

	return nil
}

// isIPForwardingEnabled checks the proc kernel setting to see if ip forwarding is enabled for the specified family
func isIPForwardingEnabled(ipForwardFilePath string) (bool, error) {
	content, err := os.ReadFile(ipForwardFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to check the linux proc filepath on this relay node: %w", err)
	}

	value := strings.TrimSpace(string(content))
	if value == "1" {
		return true, nil
	}

	return false, nil
}
