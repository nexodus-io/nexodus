package nexodus

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"go.uber.org/zap"
)

// default key pair file locations (windows needs work)
const (
	publicKeyPermissions  = 0644
	privateKeyPermissions = 0600
)

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and write them to location on the disk depending on the OS
func (ax *Nexodus) handleKeys() error {

	pubKeyFile := filepath.Join(ax.stateDir, "public.key")
	privKeyFile := filepath.Join(ax.stateDir, "private.key")

	// We used to store the keys in a different location..
	// migrate them to the state dir if needed.
	err := ax.migrationCheck(pubKeyFile, privKeyFile)
	if err != nil {
		return err
	}

	publicKey := readKeyFile(ax.logger, pubKeyFile)
	privateKey := readKeyFile(ax.logger, privKeyFile)
	if publicKey != "" && privateKey != "" {
		ax.wireguardPubKey = publicKey
		ax.wireguardPvtKey = privateKey
		ax.logger.Infof("Existing key pair found at [ %s ] and [ %s ]", pubKeyFile, privKeyFile)
		return nil
	}
	ax.logger.Infof("No existing public/private key pair found, generating a new pair")
	if err := ax.generateKeyPair(pubKeyFile, privKeyFile); err != nil {
		return fmt.Errorf("Unable to locate or generate a key/pair: %w", err)
	}
	ax.logger.Debugf("New keys were written to [ %s ] and [ %s ]", pubKeyFile, privKeyFile)
	return nil
}

// migrationCheck should not be needed after everyone has upgraded to the latest nexd.
func (ax *Nexodus) migrationCheck(pubKeyFile string, privKeyFile string) error {

	// skip if the new key files exist
	if canReadFile(pubKeyFile) && canReadFile(privKeyFile) {
		return nil
	}

	oldPubKeyFile := "public.key"
	oldPrivKeyFile := "private.key"
	if !ax.userspaceMode {
		switch runtime.GOOS {
		case "darwin":
			oldPubKeyFile = "/usr/local/etc/wireguard/public.key"
			oldPrivKeyFile = "/usr/local/etc/wireguard/private.key"
		case "windows":
			oldPubKeyFile = "C:/nexd/public.key"
			oldPrivKeyFile = "C:/nexd/private.key"
		case "linux":
			oldPubKeyFile = "/etc/wireguard/public.key"
			oldPrivKeyFile = "/etc/wireguard/private.key"
		}
	}

	// skip if they being stored in the same place.
	if pubKeyFile == oldPubKeyFile || privKeyFile == oldPrivKeyFile {
		return nil
	}

	// skip if the old key files don't exist
	if !(canReadFile(oldPubKeyFile) && canReadFile(oldPrivKeyFile)) {
		return nil
	}

	data, err := os.ReadFile(oldPubKeyFile)
	if err != nil {
		return err
	}
	err = os.WriteFile(pubKeyFile, data, publicKeyPermissions)
	if err != nil {
		return err
	}
	data, err = os.ReadFile(oldPrivKeyFile)
	if err != nil {
		return err
	}
	err = os.WriteFile(privKeyFile, data, privateKeyPermissions)
	if err != nil {
		return err
	}

	// TODO: decide if we should delete the old keys.
	//err = os.Remove(oldPubKeyFile)
	//if err != nil {
	//	return err
	//}
	//err = os.Remove(oldPrivKeyFile)
	//if err != nil {
	//	return err
	//}

	return nil
}

func canReadFile(name string) bool {
	info, err := os.Stat(name)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}

// generateKeyPair a key pair and write them to disk
func (ax *Nexodus) generateKeyPair(publicKeyFile, privateKeyFile string) error {

	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	ax.wireguardPubKey = privateKey.PublicKey().String()
	ax.wireguardPvtKey = privateKey.String()

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
