package ipsec

import (
	"fmt"
	"os/exec"
	"strings"
)

func RestartIPSecSystemd() error {
	_, err := runCommand("systemctl", "restart", "strongswan")
	if err != nil {
		return fmt.Errorf("error restarting ipsec: %v", err)
	}

	return nil
}

// StartIpsec stroke commands
func StartIpsec() error {
	_, err := runCommand("ipsec", "start")
	if err != nil {
		return fmt.Errorf("error starting ipsec: %v", err)
	}

	return nil
}

// StopIpsec stroke commands
func StopIpsec() error {
	_, err := runCommand("ipsec", "stop")
	if err != nil {
		return fmt.Errorf("error stopping ipsec: %v", err)
	}

	return nil
}

// UpdateIpsecConnections stroke commands
func UpdateIpsecConnections() error {
	_, err := runCommand("ipsec", "update")
	if err != nil {
		return fmt.Errorf("error updating ipsec connections: %v", err)
	}

	return nil
}

// ReloadIpsecConnections stroke commands
func ReloadIpsecConnections() error {
	_, err := runCommand("ipsec", "reload")
	if err != nil {
		return fmt.Errorf("error reloading ipsec connections: %v", err)
	}

	return nil
}

// RestartIpsec stroke commands
func RestartIpsec() error {
	_, err := runCommand("ipsec", "restart")
	if err != nil {
		return fmt.Errorf("error restarting ipsec: %v", err)
	}

	return nil
}

// RunCommand runs the cmd and returns the combined stdout and stderr
func runCommand(cmd ...string) (string, error) {
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %q: %s (%s)", strings.Join(cmd, " "), err, output)
	}

	return string(output), nil
}
