//go:build linux

package nexodus

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nexodus-io/nexodus/internal/util"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func (nx *Nexodus) runIpLinkAdd() (string, error) {
	if _, found := os.LookupEnv("NEXD_USE_WIREGUARD_GO"); found {
		return "", fmt.Errorf("Error: Unknown device type.")
	}
	return RunCommand("ip", "link", "add", nx.tunnelIface, "type", "wireguard")
}

// setupLinuxInterface TODO replace with netlink calls
// this is called if this is the first run or if the local node
// address got assigned a new address by the controller
func (nx *Nexodus) setupInterfaceOS() error {

	logger := nx.logger
	// delete the wireguard ip link interface if it exists
	if ifaceExists(logger, nx.tunnelIface) {
		_, err := RunCommand("ip", "link", "del", nx.tunnelIface)
		if err != nil {
			logger.Debugf("failed to delete the ip link interface: %v\n", err)
		}
	}

	if nx.TunnelIP == "" || nx.TunnelIpV6 == "" {
		return fmt.Errorf("Have not received local node address configuration from the service, returning for a retry")
	}

	// create the wireguard ip link interface
	_, err := nx.runIpLinkAdd()
	if err != nil {
		if !strings.Contains(err.Error(), "Error: Unknown device type.") {
			logger.Errorf("failed to create the ip link interface: %v\n", err)
			return fmt.Errorf("%w", interfaceErr)
		}
		// the linux kernel might not be compiled with wg support.
		// fallback to using wireguard-go

		if _, err = os.Stat("/dev/net"); err != nil {
			err = os.MkdirAll("/dev/net", 0755)
			if err != nil {
				return err
			}
		}

		if _, err = os.Stat("/dev/net/tun"); err != nil {
			_, err = RunCommand("mknod", "/dev/net/tun", "c", "10", "200")
			if err != nil {
				return err
			}
		}

		// prefer nexd-wireguard-go over wireguard-go since it supports port reuse.
		wgBinary := wgGoBinary
		if path, err := exec.LookPath(nexdWgGoBinary); err == nil {
			wgBinary = path
		}

		logger.Debugf("Creating network interface using wireguard-go")
		_, err := RunCommand(wgBinary, nx.tunnelIface)
		if err != nil {
			logger.Errorf("failed to create the %s interface: %v\n", nx.tunnelIface, err)
			return fmt.Errorf("%w", interfaceErr)
		}
	}

	listenPort := nx.listenPort
	// start the wireguard listener on a well-known port if it is a discovery or relay node as all
	// nodes need to be able to reach those services for state distribution, hole punching and relay.
	if nx.relay {
		listenPort = WgDefaultPort
	}
	privateKey, err := wgtypes.ParseKey(nx.wireguardPvtKey)
	if err != nil {
		logger.Errorf("invalid wiregaurd private key: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}
	c, err := wgctrl.New()
	if err != nil {
		logger.Errorf("could not connect to wireguard: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}
	defer util.IgnoreError(c.Close)

	wgFwMark := 51820
	err = c.ConfigureDevice(nx.tunnelIface, wgtypes.Config{
		PrivateKey:   &privateKey,
		ListenPort:   &listenPort,
		ReplacePeers: true,
		Peers:        nil,
		FirewallMark: &wgFwMark,
	})
	if err != nil {
		logger.Errorf("failed to start the wireguard listener: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}

	// assign the wg interface a v6 address
	if nx.ipv6Supported {
		localAddressIPv6 := fmt.Sprintf("%s/%s", nx.TunnelIpV6, wgOrgIPv6PrefixLen)
		_, err = RunCommand("ip", "-6", "address", "add", localAddressIPv6, "dev", nx.tunnelIface)
		if err != nil {
			logger.Infof("failed to assign an IPv6 address to the local linux ipv6 interface, ensure v6 is supported: %v\n", err)
		}
	}

	// assign the wg interface a v4 address, delete the existing if one is present
	_, err = RunCommand("ip", "address", "add", nx.TunnelIP, "dev", nx.tunnelIface)
	if err != nil {
		logger.Debugf("failed to assign an address to the local linux interface, attempting to flush the iface: %v\n", err)
		wgIP := nx.getIPv4Iface(nx.tunnelIface)
		// TODO: this is likely legacy from a push model, should be ok to remove the deletes since the agent now deletes wg0 on startup
		_, err = RunCommand("ip", "address", "del", wgIP.To4().String(), "dev", nx.tunnelIface)
		if err != nil {
			logger.Errorf("failed to assign an IPv4 address to the local linux interface: %v\n", err)
		}
		_, err = RunCommand("ip", "address", "add", nx.TunnelIP, "dev", nx.tunnelIface)
		if err != nil {
			logger.Errorf("failed to assign an address to the local linux interface: %v\n", err)
			return fmt.Errorf("%w", interfaceErr)
		}
	}
	// bring the wg0 interface up
	_, err = RunCommand("ip", "link", "set", nx.tunnelIface, "up")
	if err != nil {
		logger.Errorf("failed to bring up the wg interface: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}

	return nil
}

func (nx *Nexodus) removeExistingInterface() {
	if linkExists(nx.tunnelIface) {
		if err := delLink(nx.tunnelIface); err != nil {
			// not a fatal error since if this is on startup it could be absent
			nx.logger.Debugf("failed to delete netlink interface %s: %v", nx.tunnelIface, err)
		}
	}
}

func (nx *Nexodus) findLocalIP() (string, error) {
	// Linux network discovery
	linuxIP, err := discoverLinuxAddress(nx.logger, 4)
	if err != nil {
		return "", err
	}

	return linuxIP.String(), nil
}
