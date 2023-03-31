//go:build windows

package nexodus

import (
	"fmt"
	"os"
	"text/template"
	"time"
)

const (
	windowsConfFilePermissions = 0644
	windowsWgConfigFile        = "C:/nexd/wg0.conf"
)

func (ax *Nexodus) setupInterfaceOS() error {

	logger := ax.logger
	localAddress := ax.wgLocalAddress
	privateKey := ax.wireguardPvtKey
	dev := ax.tunnelIface

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

func (ax *Nexodus) removeExistingInterface() {
}

func (ax *Nexodus) findLocalIP() (string, error) {
	return discoverGenericIPv4(ax.logger, ax.controllerURL.Host, "443")
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
