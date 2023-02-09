package apex

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"

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

// GetOS get os type
func GetOS() (operatingSystem string) {
	return runtime.GOOS
}

// RunCommand runs the cmd and returns the combined stdout and stderr
func RunCommand(cmd ...string) (string, error) {
	// #nosec -- G204: Subprocess launched with a potential tainted input or cmd arguments
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %q: %w (%s)", strings.Join(cmd, " "), err, output)
	}
	return string(output), nil
}

// IsCommandAvailable checks to see if a binary is available in the current path
func IsCommandAvailable(name string) bool {
	if _, err := exec.LookPath(name); err != nil {
		return false
	}
	return true
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
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("'%s' is not a valid v4 or v6 IP prefix: %w", cidr, err)
	}
	return nil
}

// discoverGenericIPv4 opens a socket to the controller and returns the IP of the source dial
func discoverGenericIPv4(logger *zap.SugaredLogger, controller string, port string) (string, error) {
	controllerSocket := fmt.Sprintf("%s:%s", controller, port)
	conn, err := net.Dial("udp", controllerSocket)
	if err != nil {
		return "", err
	}
	conn.Close()
	ipAddress := conn.LocalAddr().(*net.UDPAddr)
	if ipAddress != nil {
		ipPort := strings.Split(ipAddress.String(), ":")
		logger.Debugf("Nodes discovered local address is [%s]", ipPort[0])
		return ipPort[0], nil
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
	ipAndPort, err := GetPublicUDPAddr(logger, 0)
	if err != nil {
		return false, err
	}
	if hostIP != ipAndPort.IP.String() {
		return true, nil
	}
	return false, nil
}

// CreateDirectory create a directory if one does not exist
func CreateDirectory(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create the directory %s: %w", path, err)
		}
	}
	return nil
}

func FileExists(f string) bool {
	if _, err := os.Stat(f); err != nil {
		return false
	}
	return true
}

// ParseIPNet return an IPNet from a string
func ParseIPNet(s string) (*net.IPNet, error) {
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

// writeToFile overwrite the contents of a file
func writeToFile(logger *zap.SugaredLogger, s, file string, filePermissions int) {
	// overwrite the existing file contents
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(filePermissions))
	if err != nil {
		logger.Warnf("Unable to open a key file to write to: %v", err)
	}

	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			logger.Warnf("Unable to write key to file [ %s ] %v", file, err)
		}
	}(f)

	wr := bufio.NewWriter(f)
	_, err = wr.WriteString(s)
	if err != nil {
		logger.Warnf("Unable to write key to file [ %s ] %v", file, err)
	}
	if err = wr.Flush(); err != nil {
		logger.Warnf("Unable to write key to file [ %s ] %v", file, err)
	}
}

func defaultTunnelDev(operatingSystem string) string {
	switch operatingSystem {
	case Darwin.String():
		return darwinIface
	case Linux.String():
		return wgIface
	case Windows.String():
		return wgIface
	}

	return ""
}
