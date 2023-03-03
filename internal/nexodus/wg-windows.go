package nexodus

import (
	"fmt"
	"os"
	"text/template"
)

const (
	windowsConfFilePermissions = 0644
	windowsWgConfigFile        = "C:/nexd/wg0.conf"
)

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
