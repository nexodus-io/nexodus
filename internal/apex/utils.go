package apex

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

// supported OS types
type OperatingSystem string

const (
	Linux   OperatingSystem = "Linux"
	Darwin  OperatingSystem = "Darwin"
	Windows OperatingSystem = "Windows"
)

func (operatingSystem OperatingSystem) String() string {
	switch operatingSystem {
	case Linux:
		return "linux"
	case Darwin:
		return "darwin"
	case Windows:
		return "windows"
	}

	return "unsupported"
}

// GetOS get os type
func GetOS() (operatingSystem string) {
	return runtime.GOOS
}

// RunCommand runs the cmd and returns the combined stdout and stderr
func RunCommand(cmd ...string) (string, error) {
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %q: %s (%s)", strings.Join(cmd, " "), err, output)
	}
	return string(output), nil
}

// IsCommandAvailable checks to see if a binary is available in the current path
func IsCommandAvailable(name string) bool {
	if _, err := exec.LookPath(name); err != nil {
		return false
	}
	return true
}

// ValidateIp ensures a valid IP4/IP6 address is provided
func ValidateIp(ip string) error {
	if ip := net.ParseIP(ip); ip != nil {
		return nil
	}
	return fmt.Errorf("%s is not a valid v4 or v6 IP", ip)
}

// ValidateCIDR ensures a valid IP4/IP6 prefix is provided
func ValidateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("%s is not a valid v4 or v6 IP prefix", err)
	}
	return nil
}

func FileToString(file string) string {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		log.Errorf("unable to read the file [%s] %v\n", file, err)
		return ""
	}
	return string(fileContent)
}

// CopyFile source destination
func CopyFile(src, dst string) error {
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

// discoverGenericIPv4 opens a socket to the controller and returns the IP of the source dial
func discoverGenericIPv4(controller string, port int) (string, error) {
	controllerSocket := fmt.Sprintf("%s:%d", controller, port)
	conn, err := net.Dial("udp", controllerSocket)
	if err != nil {
		return "", err
	}
	conn.Close()
	ipAddress := conn.LocalAddr().(*net.UDPAddr)
	if ipAddress != nil {
		ipPort := strings.Split(ipAddress.String(), ":")
		log.Debugf("Nodes disovered local address is [%s]", ipPort[0])
		return ipPort[0], nil
	}
	return "", fmt.Errorf("failed obtain the local IP")
}

// GetPubIP retrieves current global IP address using https://checkip.amazonaws.com/
func GetPubIP() (string, error) {
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
	ip, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response from https://checkip.amazonaws.com: %w", err)
	}
	return strings.TrimSpace(string(ip)), nil
}

func IsNAT(nodeOS, controller string, port int) (bool, error) {
	var hostIP string
	var err error
	if nodeOS == Darwin.String() {
		hostIP, err = discoverGenericIPv4(controller, port)
		if err != nil {
			return false, err
		}
	}
	if nodeOS == Windows.String() {
		hostIP, err = discoverGenericIPv4(controller, port)
		if err != nil {
			return false, err
		}
	}
	if nodeOS == Linux.String() {
		linuxIP, err := discoverLinuxAddress(4)
		if err != nil {
			return false, err
		}
		hostIP = linuxIP.String()
	}
	pubIP, err := GetPubIP()
	if err != nil {
		return false, err
	}
	if hostIP != pubIP {
		return true, nil
	}
	return false, nil
}

// CreateDirectory create a directory if one does not exist
func CreateDirectory(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create the directory %s: %v", path, err)
		}
	}
	return nil
}

func FileExists(f string) bool {
	if _, err := os.Stat(f); err != nil {
		return false
	}
	return true
}

// ReadKeyFileToString reads the key file and strips any newline chars that create wireguard issues
func ReadKeyFileToString(s string) (string, error) {
	buf, err := os.ReadFile(s)
	if err != nil {
		return "", fmt.Errorf("unable to read file: %v\n", err)
	}
	rawStr := string(buf)
	return strings.Replace(rawStr, "\n", "", -1), nil
}

// ParseIPNet return an IPNet from a string
func ParseIPNet(s string) (*net.IPNet, error) {
	ip, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	return &net.IPNet{IP: ip, Mask: ipNet.Mask}, nil
}

// sanitizeWindowsConfig removes incompatible fields from the wg Interface section
func sanitizeWindowsConfig(file string) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalf("Unable to read the wg0 configuration file %s: %v", file, err)
	}
	post := regexp.MustCompile("(?m)[\r\n]+^.*Post.*$")
	regOut := post.ReplaceAllString(string(b), "")
	post = regexp.MustCompile("(?m)[\r\n]+^.*SaveConfig.*$")
	regOut = post.ReplaceAllString(regOut, "")
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("Unable to open to the wg0 configuration file %s: %v", file, err)
	}
	_, err = f.Write([]byte(regOut))
	if err != nil {
		log.Fatalf("Unable to open to the wg0 configuration file %s: %v", file, err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("Unable to write to the wg0 configuration file %s: %v", file, err)
	}
}
