package main

import (
	"fmt"
	"os"
	"runtime"
)

// hasPrivileges checks for root/admin privileges based on the platform.
func hasPrivileges() error {
	switch osType := getOSType(); osType {
	case "linux":
		if !isLinuxRoot() {
			return fmt.Errorf("'nexctl nexd' commands must be run with sudo on Linux")
		}
	case "darwin":
		if !isDarwinRoot() {
			return fmt.Errorf("'nexctl nexd' commands must be run with sudo on macOS")
		}
	case "windows":
		if !isWindowsAdmin() {
			return fmt.Errorf("'nexctl nexd' commands must be run with administrator privileges on Windows")
		}
	default:
		return fmt.Errorf("unsupported operating system type: %s", osType)
	}
	return nil
}

// getOSType gets the operating system.
func getOSType() string {
	switch os := runtime.GOOS; os {
	case "linux":
		return "linux"
	case "darwin":
		return "darwin"
	case "windows":
		return "windows"
	default:
		return "unknown"
	}
}

// isLinuxRoot checks if the program is running as root on Linux.
func isLinuxRoot() bool {
	return os.Geteuid() == 0
}

// isDarwinRoot checks if the program is running as root on macOS.
func isDarwinRoot() bool {
	return os.Geteuid() == 0
}

// isWindowsAdmin checks if the program is running with administrative privileges on Windows.
func isWindowsAdmin() bool {
	// Check for admin privileges on Windows.
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}
