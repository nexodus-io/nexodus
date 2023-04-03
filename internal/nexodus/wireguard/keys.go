package wireguard

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// default key pair file locations (windows needs work)
const (
	workdirPublicKeyFile  = "public.key"
	workdirPrivateKeyFile = "private.key"
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
func (wg *WireGuard) generateKeyPair(publicKeyFile, privateKeyFile string) error {

	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	wg.WireguardPubKey = privateKey.PublicKey().String()
	wg.WireguardPvtKey = privateKey.String()

	// TODO remove this debug statement at some point
	wg.Logger.Debugf("Public Key [ %s ] Private Key [ %s ]", wg.WireguardPubKey, wg.WireguardPvtKey)
	// write the new keys to disk
	WriteToFile(wg.Logger, wg.WireguardPubKey, publicKeyFile, publicKeyPermissions)
	WriteToFile(wg.Logger, wg.WireguardPvtKey, privateKeyFile, privateKeyPermissions)

	return nil
}

// readKeyFile reads the contents of a key file
func readKeyFile(logger *zap.SugaredLogger, keyFile string) string {
	if !fileExists(keyFile) {
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
