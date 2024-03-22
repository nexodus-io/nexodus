package main

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

// hasPrivileges checks to see if we can access nexd over the admin interface (unix socket)
func hasPrivileges() error {

	// the true test if we have privileges is if we can open the socket file.
	err := canAccessSocketAPI()
	if err == nil {
		return nil
	}

	// If we can't open it's likely we are not running as root/admin.  Return a helpful error message....
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

	// we are not running as root/admin and we can't open the socket file.  It's likely nexd is not running.
	return fmt.Errorf("is nexd running?: %w", err)
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

func canAccessSocketAPI() error {
	conn, err := net.Dial("unix", api.UnixSocketPath)
	if err != nil {
		conn, err = net.Dial("unix", filepath.Base(api.UnixSocketPath))
		if err != nil {
			return fmt.Errorf("failed to connect to nexd at '%s': %w\n", api.UnixSocketPath, err)
		}
	}
	conn.Close()
	return nil
}
