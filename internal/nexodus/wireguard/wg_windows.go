//go:build windows

package wireguard

import (
	"fmt"
	"net"
	"os"
	"text/template"
	"time"
)

const (
	windowsConfFilePermissions = 0644
	windowsWgConfigFile        = "C:/nexd/wg0.conf"
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
	privateKey := wg.WireguardPvtKey
	dev := wg.TunnelIface

	if err := buildWindowsWireguardIfaceConf(privateKey, localAddress); err != nil {
		logger.Errorf("failed to create the windows wireguard wg0 interface file: %w", err)
		return fmt.Errorf("%w", interfaceErr)
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
		logger.Errorf("failed to start the wireguard interface: %w", err)
		return fmt.Errorf("%w", interfaceErr)
	}
	logger.Debugf("started windows tunnel svc: %v\n", wgOut)
	// ensure the link is created before adding peers
	if !ifaceExists(logger, dev) {
		logger.Debugf("windows iface %s has not been created yet, pausing for 1 second", dev)
		time.Sleep(time.Second * 1)
	}
	// fatal out if the interface is not created
	if !ifaceExists(logger, dev) {
		logger.Errorf("failed to create the windows wireguard interface: %w", err)
		return fmt.Errorf("%w", interfaceErr)
	}

	return nil
}

func (wg *WireGuard) RemoveExistingInterface() {
	ifaces, err := net.Interfaces()
	if err != nil {
		wg.Logger.Warnf("localAddresses: %w\n", err)
	}
	for _, iface := range ifaces {
		if iface.Name == wg.TunnelIface {
			wg.Logger.Debugf("Existing wireguard interface %s found.", iface.Name)
			//TODO: Implement delete interface
		}
	}
}

func buildWindowsWireguardIfaceConf(pvtKey, wgAddress string) error {
	f, err := fileHandle(windowsWgConfigFile, windowsConfFilePermissions)
	if err != nil {
		return err
	}

	defer f.Close()

	t := template.Must(template.New("wgconf").Parse(windowsIfaceConfig))
	if err := t.Execute(f, struct {
		PrivateKey string
		WgAddress  string
	}{
		PrivateKey: pvtKey,
		WgAddress:  wgAddress,
	}); err != nil {
		return fmt.Errorf("failed to fill windows template %s: %w", windowsWgConfigFile, err)
	}

	return nil
}

func fileHandle(file string, permissions int) (*os.File, error) {
	var f *os.File
	var err error
	f, err = os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(permissions))
	if err != nil {
		return nil, err
	}

	return f, nil
}

var windowsIfaceConfig = `
[Interface]
PrivateKey = {{ .PrivateKey }}
Address = {{ .WgAddress }}
`
