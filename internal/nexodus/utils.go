package nexodus

import (
	"fmt"
	"net"
	"os/exec"

	"go.uber.org/zap"
)

// OperatingSystem supported OS types
type OperatingSystem string

const (
	Linux   OperatingSystem = "Linux"
	Darwin  OperatingSystem = "Darwin"
	Windows OperatingSystem = "Windows"
)

func (operatingSystem OperatingSystem) String() string {
	switch operatingSystem {
	case Linux:
		return "linux"
	case Darwin:
		return "darwin"
	case Windows:
		return "windows"
	}

	return "unsupported"
}

// ValidateIp ensures a valid IP4/IP6 address is provided
func ValidateIp(ip string) error {
	if ip := net.ParseIP(ip); ip != nil {
		return nil
	}
	return fmt.Errorf("%s is not a valid v4 or v6 IP", ip)
}

// ValidateCIDR ensures a valid IP4/IP6 prefix is provided
func ValidateCIDR(cidr string) error {
	_, netAddr, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("'%s' is not a valid v4 or v6 IP prefix: %w", cidr, err)
	}

	if cidr != netAddr.String() {
		return fmt.Errorf("Invalid network prefix provided %s, try using %s\n", cidr, netAddr.String())
	}

	return nil
}

// ParseIPNet return an IPNet from a string
func parseIPNet(s string) (*net.IPNet, error) {
	ip, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	return &net.IPNet{IP: ip, Mask: ipNet.Mask}, nil
}

func parseNetworkStr(cidr string) (string, error) {
	_, nw, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}
	return nw.String(), nil
}

func LocalIPv4Address() net.IP {
	interfaces, _ := net.Interfaces()
	for _, inter := range interfaces {
		addrs, err := inter.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch ip := addr.(type) {
			case *net.IPNet:
				if ip.IP.IsLoopback() {
					continue
				}
				if ip.IP.DefaultMask() == nil {
					continue
				}
				return ip.IP
			}
		}
	}
	return nil
}

// relayIpTables iptables for the relay node
func relayIpTables(logger *zap.SugaredLogger, dev string) {
	_, err := exec.Command("iptables", "-A", "FORWARD", "-i", dev, "-j", "ACCEPT").CombinedOutput()
	if err != nil {
		logger.Warnf("the hub router iptables rule was not added: %v", err)
	}
}

// discoverGenericIPv4 opens a socket to the controller and returns the IP of the source dial
func discoverGenericIPv4(logger *zap.SugaredLogger, controller string, port string) (string, error) {
	controllerSocket := fmt.Sprintf("%s:%s", controller, port)
	conn, err := net.Dial("udp4", controllerSocket)
	if err != nil {
		return "", err
	}
	conn.Close()
	ipAddress := conn.LocalAddr().(*net.UDPAddr)
	if ipAddress != nil {
		ip, _, err := net.SplitHostPort(ipAddress.String())
		if err != nil {
			return "", err
		}
		logger.Debugf("Nodes discovered local address is [%s]", ip)
		return ip, nil
	}
	return "", fmt.Errorf("failed to obtain the local IP")
}

func IsNAT(logger *zap.SugaredLogger, nodeOS, controller string, port string) (bool, error) {
	var hostIP string
	var err error
	if nodeOS == Darwin.String() || nodeOS == Windows.String() {
		hostIP, err = discoverGenericIPv4(logger, controller, port)
		if err != nil {
			return false, err
		}
	}
	if nodeOS == Linux.String() {
		linuxIP, err := discoverLinuxAddress(logger, 4)
		if err != nil {
			return false, err
		}
		hostIP = linuxIP.String()
	}
	ipAndPort, err := stunRequest(logger, stunServer1, 0)
	if err != nil {
		return false, err
	}
	if hostIP != ipAndPort.IP.String() {
		return true, nil
	}
	return false, nil
}
