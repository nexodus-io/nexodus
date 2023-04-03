//go:build darwin

package wireguard

import (
	"fmt"
	"os/exec"

	"go.uber.org/zap"
)

func defaultTunnelDevOS() string {
	return darwinIface
}

// binaryChecks validate the required binaries are available
func binaryChecks() error {
	// Darwin wireguard-go userspace binary
	if _, err := exec.LookPath(wgGoBinary); err != nil {
		return fmt.Errorf("%s command not found, is wireguard installed?", wgGoBinary)
	}
	return nil
}

// checkOS and report error if the OS is not supported.
func checkOS(logger *zap.SugaredLogger) error {
	// ensure the osx wireguard directory exists
	if err := CreateDirectory(WgDarwinConfPath); err != nil {
		return fmt.Errorf("unable to create the wireguard config directory [%s]: %w", WgDarwinConfPath, err)
	}
	return nil
}
