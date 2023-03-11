package nexodus

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// default key pair file locations (windows needs work)
const (
	linuxPublicKeyFile    = "/etc/wireguard/public.key"
	linuxPrivateKeyFile   = "/etc/wireguard/private.key"
	darwinPublicKeyFile   = "/usr/local/etc/wireguard/public.key"
	darwinPrivateKeyFile  = "/usr/local/etc/wireguard/private.key"
	windowsPublicKeyFile  = "C:/nexd/public.key"
	windowsPrivateKeyFile = "C:/nexd/private.key"
	publicKeyPermissions  = 0644
	privateKeyPermissions = 0600
)

// generateKeyPair a key pair and write them to disk
func (ax *Nexodus) generateKeyPair(publicKeyFile, privateKeyFile string) error {
	cmd := exec.Command(wgBinary, "genkey")
	privateKey, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("wg genkey error: %w", err)
	}

	cmd = exec.Command(wgBinary, "pubkey")
	cmd.Stdin = bytes.NewReader(privateKey)
	publicKey, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("wg pubkey error: %w", err)
	}
	ax.wireguardPubKey = strings.TrimSpace(string(publicKey))
	ax.wireguardPvtKey = strings.TrimSpace(string(privateKey))

	// TODO remove this debug statement at some point
	ax.logger.Debugf("Public Key [ %s ] Private Key [ %s ]", ax.wireguardPubKey, ax.wireguardPvtKey)
	// write the new keys to disk
	WriteToFile(ax.logger, ax.wireguardPubKey, publicKeyFile, publicKeyPermissions)
	WriteToFile(ax.logger, ax.wireguardPvtKey, privateKeyFile, privateKeyPermissions)

	return nil
}

// readKeyFile reads the contents of a key file
func readKeyFile(logger *zap.SugaredLogger, keyFile string) string {
	if !FileExists(keyFile) {
		return ""
	}
	key, err := readKeyFileToString(keyFile)
	if err != nil {
		logger.Debugf("unable to read key file: %v", err)
		return ""
	}

	return key
}

// readKeyFileToString reads the key file and strips any newline chars that create wireguard issues
func readKeyFileToString(s string) (string, error) {
	buf, err := os.ReadFile(s)
	if err != nil {
		return "", fmt.Errorf("unable to read file: %w", err)
	}
	rawStr := string(buf)
	return strings.Replace(rawStr, "\n", "", -1), nil
}
