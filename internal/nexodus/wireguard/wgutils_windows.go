//go:build windows

package wireguard

import (
	"fmt"
	"os/exec"

	"go.uber.org/zap"
)

func defaultTunnelDevOS() string {
	return wgIface
}

// binaryChecks validate the required binaries are available
func binaryChecks() error {
	if _, err := exec.LookPath(wgWinBinary); err != nil {
		return fmt.Errorf("%s command not found, is wireguard installed?", wgWinBinary)
	}
	return nil
}

// checkOS and report error if the OS is not supported.
func checkOS(logger *zap.SugaredLogger) error {
	// ensure the windows wireguard directory exists
	if err := CreateDirectory(WgWindowsConfPath); err != nil {
		return fmt.Errorf("unable to create the wireguard config directory [%s]: %w", WgWindowsConfPath, err)
	}
	return nil
}
