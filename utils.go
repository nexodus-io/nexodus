package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Encapsulation supported encap types
type Encapsulation string

const (
	Wireguard Encapsulation = "wireguard"
)

func (encap Encapsulation) String() string {
	switch encap {
	case Wireguard:
		return "wireguard"
	}
	return "unsupported"
}

// supported OS types
type operatingSystem string

const (
	linux  operatingSystem = "linux"
	darwin operatingSystem = "darwin"
)

func (operatingSystem operatingSystem) String() string {
	switch operatingSystem {
	case linux:
		return "linux"
	case darwin:
		return "darwin"
	}

	return "unsupported"
}

// GetOS get os type
func getOS() (operatingSystem string) {
	return runtime.GOOS
}

// runCommand runs the cmd and returns the combined stdout and stderr
func runCommand(cmd ...string) (string, error) {
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %q: %s (%s)", strings.Join(cmd, " "), err, output)
	}
	return string(output), nil
}

// isCommandAvailable checks to see if a binary is available in the current path
func isCommandAvailable(name string) bool {
	if _, err := exec.LookPath(name); err != nil {
		return false
	}
	return true
}

// timestampFile return a unique timestamped filename
func timestampFile(filename string) string {
	return fmt.Sprintf(filename + "-" + time.Now().Format("20060102150405"))
}

// SplitTunnelDest is used to parse node name from
// node IP if multiple tunnel endpoints are passed
func SplitTunnelDest(tunnelDestInput string) []string {
	return strings.Split(tunnelDestInput, ",")
}

// SplitTunnelPair splits the IP and hostname
// todo: fix this as it probably breaks with ipv6
func splitTunnelPair(tunnelDestInput string) []string {
	return strings.Split(tunnelDestInput, ":")
}

// mapTunnelDest creates a k/v pair of node name and node IP
func mapTunnelDest(tunnelDestInput string) map[string]net.IP {
	tunnelDestList := strings.Split(tunnelDestInput, ",")
	tunnelDestMap := make(map[string]net.IP)
	for _, tunnelDestPair := range tunnelDestList {
		hostAddressPair := splitTunnelPair(tunnelDestPair)
		// validate the passed remote address is a valid v4 or v6 address
		if err := validateIp(hostAddressPair[1]); err != nil {
			log.Fatal(err)
		}
		tunnelDestMap[hostAddressPair[0]] = net.ParseIP(hostAddressPair[1])
	}
	return tunnelDestMap
}

// validateIp ensures a valid IP4/IP6 address is provided
func validateIp(ip string) error {
	if ip := net.ParseIP(ip); ip != nil {
		return nil
	}
	return fmt.Errorf("%s is not a valid v4 or v6 IP", ip)
}

// diffConfig diffs the contents of two files
func diffConfig(oldCfg, newCfg string) bool {
	cfgOld, err := ioutil.ReadFile(oldCfg)
	if err != nil {
		log.Fatalf("unable to read file: %v", err)
	}
	cfgNew, err := ioutil.ReadFile(newCfg)
	if err != nil {
		log.Fatalf("unable to read file: %v", err)
	}
	if string(cfgOld) != string(cfgNew) {
		return false
	}
	return true
}

// copyFile source destination
func copyFile(src, dst string) error {
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(w, r)
	return err
}

// getIPv4 returns the first address from hostname -I
func getIPv4() (string, error) {
	out, err := runCommand("hostname", "-I")
	if err != nil {
		return "", err
	}
	items := strings.Split(string(out), " ")
	if len(items) == 0 {
		return "", fmt.Errorf("failed to exec hostname")
	}

	return items[0], nil
}

// getDarwinIPv4 returns the first address from ipconfig getifaddr en0
func getDarwinIPv4() (string, error) {
	osxIP, err := runCommand("ipconfig", "getifaddr", "en0")
	if err != nil {
		return "", err
	}
	return osxIP, nil
}

// getPubIP retrieves current global IP address using https://checkip.amazonaws.com/
func getPubIP() (string, error) {
	c := http.DefaultClient
	req, err := http.NewRequest("GET", "https://checkip.amazonaws.com/", nil)
	if err != nil {
		return "", err
	}
	res, err := c.Do(req)
	if err != nil {
		return "", err
	}
	if res.StatusCode >= http.StatusBadRequest {
		defer func() {
			_ = res.Body.Close()
		}()
		return "", fmt.Errorf("%s: %s %s", res.Status, req.Method, req.URL)
	}
	ip, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response from https://checkip.amazonaws.com: %w", err)
	}
	return strings.TrimSpace(string(ip)), nil
}

func isNAT(nodeOS string) (bool, error) {
	var hostIP string
	var err error
	if nodeOS == darwin.String() {
		hostIP, err = getDarwinIPv4()
		if err != nil {
			return false, err
		}
	}
	if nodeOS == linux.String() {
		hostIP, err = getIPv4()
		if err != nil {
			return false, err
		}
	}
	pubIP, err := getPubIP()
	if err != nil {
		return false, err
	}
	if hostIP != pubIP {
		return true, nil
	}
	return false, nil
}

// createDirectory create a directory if one does not exist
func createDirectory(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create the directory %s: %v", path, err)
		}
	}
	return nil
}
