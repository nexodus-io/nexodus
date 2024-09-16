//go:build darwin

package nexodus

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

// RouteExistsOS checks to see if a route exists for the specified prefix
func RouteExistsOS(prefix string) (bool, error) {
	if err := ValidateCIDR(prefix); err != nil {
		return false, err
	}

	r, w, err := os.Pipe()
	if err != nil {
		return true, err
	}
	defer r.Close()
	defer w.Close()
	ns := exec.Command("netstat", "-r", "-n")
	ns.Stdout = w
	if err = ns.Start(); err != nil {
		return true, err
	}
	defer func() {
		_ = ns.Wait()
	}()

	// #nosec -- G204: Subprocess launched with a potential tainted input or cmd arguments (gosec)
	awk := exec.Command("awk", "-v", fmt.Sprintf("ip=%s", prefix), "$1 == ip {print $1}")
	awk.Stdin = r
	var output bytes.Buffer
	awk.Stdout = &output

	// Validate the IP we're expecting is in the output
	return strings.Contains(output.String(), prefix), nil
}

// AddRoute adds a route to the specified interface
func AddRoute(prefix, dev string) error {
	_, err := RunCommand("route", "-q", "-n", "add", "-inet", prefix, "-interface", dev)
	if err != nil {
		return fmt.Errorf("v4 route add failed: %w", err)
	}

	return nil
}

// AddRouteV6 adds a route to the specified interface
func AddRouteV6(prefix, dev string) error {
	_, err := RunCommand("route", "-q", "-n", "add", "-inet6", prefix, "-interface", dev)
	if err != nil {
		return fmt.Errorf("v6 route add failed: %w", err)
	}

	return nil
}

// discoverLinuxAddress only used for darwin build purposes
func discoverLinuxAddress(logger *zap.SugaredLogger, family int) (net.IP, error) {
	return nil, nil
}

// deleteIface checks to see if  is an interface exists and deletes it
func linkExists(ifaceName string) bool {
	if _, err := netlink.LinkByName(ifaceName); err != nil {
		return false
	}
	return true
}

// delLink deletes the link and assumes it exists
func delLink(ifaceName string) error {
	if link, err := netlink.LinkByName(ifaceName); err == nil {
		if err = netlink.LinkDel(link); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRoute deletes a darwin route for an ipv4 prefix
func DeleteRoute(prefix, dev string) error {
	_, err := RunCommand("route", "-q", "-n", "delete", "-inet", prefix, "-interface", dev)
	if err != nil {
		return fmt.Errorf("no route deleted: %w", err)
	}

	return nil
}

// DeleteRouteV6 deletes a darwin route for an ipv6 prefix
func DeleteRouteV6(prefix, dev string) error {
	_, err := RunCommand("route", "-q", "-n", "delete", "-inet6", prefix, "-interface", dev)
	if err != nil {
		return fmt.Errorf("no route deleted: %w", err)
	}

	return nil
}

func defaultTunnelDevOS() string {
	return darwinIface
}

func isIfaceInUse(iface net.Interface) bool {
	cmd := exec.Command("scutil", "--nwi", iface.Name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	outputStr := string(output)
	return strings.Contains(outputStr, "No network information")
}
func isIfaceTUN(iface net.Interface) bool {
	match, _ := regexp.MatchString(`^utun[0-9]*$`, iface.Name)
	return match
}
func avaliableTunnelDevOS() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Errorf("error retrieving network ifaces: %w", err)
	}
	var avaliableIface string
	for _, iface := range ifaces {
		if isIfaceTUN(iface) && isIfaceInUse(iface) {
			avaliableIface = iface.Name
			break
		}
	}
	if avaliableIface == "" {
		fmt.Errorf("no unused network interface found: %w", err)
	}
	return avaliableIface
}

// binaryChecks validate the required binaries are available
func binaryChecks() error {
	// Darwin wireguard-go userspace binary
	if IsCommandAvailable(nexdWgGoBinary) || IsCommandAvailable(wgGoBinary) {
		return nil
	}
	return fmt.Errorf("%s command not found, is wireguard installed?", wgGoBinary)
}

// isIPv6Supported returns true if the platform supports IPv6, return true if ifconfig isn't present for whatever reason
func isIPv6Supported() bool {
	res, err := RunCommand("ifconfig")
	if err != nil {
		return true
	}
	if !strings.Contains(res, "inet6") {
		return false
	}

	return true
}

// getDefaultGatewayIPv4 not currently implemented for darwin
func getDefaultGatewayIPv4() (string, error) {
	return "", fmt.Errorf("method currently unsupported for darwin")
}

// isElevatedUnix checks that nexd was started with appropriate permissions for Unix-based OS mode (Linux/macOS)
func isElevated() (bool, error) {
	if os.Geteuid() != 0 {
		return false, fmt.Errorf("nexd OS mode requires elevated privileges, please run again with sudo")
	}
	return true, nil
}
