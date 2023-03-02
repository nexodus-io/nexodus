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
	windowsPublicKeyFile  = "C:/apex/public.key"
	windowsPrivateKeyFile = "C:/apex/private.key"
	publicKeyPermissions  = 0644
	privateKeyPermissions = 0600
)

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and write them to location on the disk depending on the OS
func (ax *Apex) handleKeys() error {
	switch ax.os {
	case Darwin.String():
		publicKey := readKeyFile(ax.logger, darwinPublicKeyFile)
		privateKey := readKeyFile(ax.logger, darwinPrivateKeyFile)
		if publicKey != "" && privateKey != "" {
			ax.wireguardPubKey = publicKey
			ax.wireguardPvtKey = privateKey
			ax.logger.Infof("Existing key pair found at [ %s ] and [ %s ]", darwinPublicKeyFile, darwinPrivateKeyFile)
			return nil
		}
		ax.logger.Infof("No existing public/private key pair found, generating a new pair")
		if err := ax.generateKeyPair(darwinPublicKeyFile, darwinPrivateKeyFile); err != nil {
			return fmt.Errorf("Unable to locate or generate a key/pair: %w", err)
		}
		ax.logger.Debugf("New keys were written to [ %s ] and [ %s ]", darwinPublicKeyFile, darwinPrivateKeyFile)
		return nil

	case Windows.String():
		publicKey := readKeyFile(ax.logger, windowsPublicKeyFile)
		privateKey := readKeyFile(ax.logger, windowsPrivateKeyFile)
		if publicKey != "" && privateKey != "" {
			ax.wireguardPubKey = publicKey
			ax.wireguardPvtKey = privateKey
			ax.logger.Infof("Existing key pair found at [ %s ] and [ %s ]", windowsPublicKeyFile, windowsPrivateKeyFile)
			return nil
		}
		ax.logger.Infof("No existing public/private key pair found, generating a new pair")
		if err := ax.generateKeyPair(windowsPublicKeyFile, windowsPrivateKeyFile); err != nil {
			return fmt.Errorf("Unable to locate or generate a key/pair: %w", err)
		}
		ax.logger.Debugf("New keys were written to [ %s ] and [ %s ]", windowsPublicKeyFile, windowsPrivateKeyFile)
		return nil

	case Linux.String():
		publicKey := readKeyFile(ax.logger, linuxPublicKeyFile)
		privateKey := readKeyFile(ax.logger, linuxPrivateKeyFile)
		if publicKey != "" && privateKey != "" {
			ax.wireguardPubKey = publicKey
			ax.wireguardPvtKey = privateKey
			ax.logger.Infof("Existing key pair found at [ %s ] and [ %s ]", linuxPublicKeyFile, linuxPrivateKeyFile)
			return nil
		}
		ax.logger.Infof("No existing public/private key pair found, generating a new pair")
		if err := ax.generateKeyPair(linuxPublicKeyFile, linuxPrivateKeyFile); err != nil {
			return fmt.Errorf("Unable to locate or generate a key/pair: %w", err)
		}
		ax.logger.Debugf("New keys were written to [ %s ] and [ %s ]", linuxPublicKeyFile, linuxPrivateKeyFile)
		return nil
	}
	return nil
}

// generateKeyPair a key pair and write them to disk
func (ax *Apex) generateKeyPair(publicKeyFile, privateKeyFile string) error {
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
	writeToFile(ax.logger, ax.wireguardPubKey, publicKeyFile, publicKeyPermissions)
	writeToFile(ax.logger, ax.wireguardPvtKey, privateKeyFile, privateKeyPermissions)

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
