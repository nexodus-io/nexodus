//go:build darwin

package wireguard

import (
	"fmt"
	"go.uber.org/zap"
)

func (wg *WireGuard) setupInterface() error {
	if wg.UserspaceMode {
		return wg.setupInterfaceUS()
	}
	return wg.setupInterfaceOS(wg.Relay)
}

func (wg *WireGuard) setupInterfaceOS(relay bool) error {

	logger := wg.Logger
	localAddress := wg.WgLocalAddress
	dev := wg.TunnelIface

	if wg.ifaceExists() {
		deleteDarwinIface(logger, dev)
	}

	_, err := runCommand("wireguard-go", dev)
	if err != nil {
		logger.Errorf("failed to create the %s interface: %v\n", dev, err)
		return fmt.Errorf("%w", interfaceErr)
	}

	_, err = runCommand("ifconfig", dev, "inet", localAddress, localAddress, "alias")
	if err != nil {
		logger.Errorf("failed to assign an address to the local osx interface: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}

	_, err = runCommand("ifconfig", dev, "up")
	if err != nil {
		logger.Errorf("failed to bring up the %s interface: %v\n", dev, err)
		return fmt.Errorf("%w", interfaceErr)
	}

	_, err = runCommand("wg", "set", dev, "private-key", darwinPrivateKeyFile)
	if err != nil {
		logger.Errorf("failed to start the wireguard listener: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}

	return nil
}

func (wg *WireGuard) RemoveExistingInterface() {
	if wg.ifaceExists() {
		deleteDarwinIface(wg.Logger, wg.TunnelIface)
	}
}

// deleteDarwinIface delete the darwin userspace wireguard interface
func deleteDarwinIface(logger *zap.SugaredLogger, dev string) {
	tunSock := fmt.Sprintf("/var/run/wireguard/%s.sock", dev)
	_, err := runCommand("rm", "-f", tunSock)
	if err != nil {
		logger.Debugf("failed to delete darwin interface: %v", err)
	}
	// /var/run/wireguard/wg0.name doesnt currently exist since utun8 isnt mapped to wg0 (fails silently)
	wgName := fmt.Sprintf("/var/run/wireguard/%s.name", dev)
	_, err = runCommand("rm", "-f", wgName)
	if err != nil {
		logger.Debugf("failed to delete darwin interface: %v", err)
	}
}
