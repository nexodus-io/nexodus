//go:build windows

package nexodus

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"text/template"
	"time"
)

var (
	interfaceErr = errors.New("interface setup error")
)

const (
	windowsConfFilePermissions = 0644
	windowsWgConfigFile        = "C:/nexd/wg0.conf"
)

func (nx *Nexodus) setupInterfaceOS() error {

	dev := nx.tunnelIface
	listenPortStr := strconv.Itoa(nx.listenPort)

	if err := buildWindowsWireguardIfaceConf(nx.wireguardPvtKey, nx.TunnelIP, listenPortStr); err != nil {
		return fmt.Errorf("failed to create the windows wireguard wg0 interface file: %w", err)
	}

	var err error
	if ifaceExists(nx.logger, dev) {
		_, err = RunCommand("wireguard.exe", "/uninstalltunnelservice", dev)
		if err != nil {
			nx.logger.Debugf("failed to down the wireguard interface (this is generally ok): %v", err)
		}
		if ifaceExists(nx.logger, dev) {
			nx.logger.Debugf("existing windows iface %s has not been torn down yet", dev)
			time.Sleep(time.Second * 1)
		}
	}
	// sleep for one second to give the wg async exe time to tear down any existing wg0 configuration
	_, err = RunCommand("wireguard.exe", "/installtunnelservice", windowsWgConfigFile)
	if err != nil {
		return fmt.Errorf("failed to start the wireguard interface: %w", err)
	}
	nx.logger.Debugf("started windows tunnel service")
	// ensure the link is created before adding peers
	if !ifaceExists(nx.logger, dev) {
		time.Sleep(time.Second * 1)
	}
	// fatal out if the interface is not created
	if !ifaceExists(nx.logger, dev) {
		return fmt.Errorf("failed to create the windows wireguard interface: %w", err)
	}

	return nil
}

func (nx *Nexodus) removeExistingInterface() {
	wgOut, err := RunCommand("wireguard.exe", "/uninstalltunnelservice", wgIface)
	if err != nil {
		nx.logger.Debugf("failed to down the wireguard interface (this is generally ok): %w", err)
	}

	nx.logger.Debugf("stopped windows tunnel svc:%v\n", wgOut)
}

func (nx *Nexodus) findLocalIP() (string, error) {
	return discoverGenericIPv4(nx.logger, nx.apiURL.Host, "443")
}

func (nx *Nexodus) configureLoopback(ip string) error {
	return nil
}

func buildWindowsWireguardIfaceConf(pvtKey, wgAddress, wgListenPort string) error {
	f, err := fileHandle(windowsWgConfigFile, windowsConfFilePermissions)
	if err != nil {
		return err
	}

	defer f.Close()

	t := template.Must(template.New("wgconf").Parse(windowsIfaceConfig))
	if err := t.Execute(f, struct {
		PrivateKey   string
		WgAddress    string
		WgListenPort string
	}{
		PrivateKey:   pvtKey,
		WgAddress:    wgAddress,
		WgListenPort: wgListenPort,
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
ListenPort = {{ .WgListenPort }}
`
