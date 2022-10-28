package apex

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

// default key pair file locations (windows needs work)
const (
	linuxPublicKeyFile    = "/etc/wireguard/public.key"
	linuxPrivateKeyFile   = "/etc/wireguard/private.key"
	darwinPublicKeyFile   = "/usr/local/etc/wireguard/public.key"
	darwinPrivateKeyFile  = "/usr/local/etc/wireguard/private.key"
	windowsPublicKeyFile  = "C:/public.key"
	windowsPrivateKeyFile = "C:/private.key"
	publicKeyPermissions  = 0644
	privateKeyPermissions = 0600
)

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and write them to location on the disk depending on the OS
func (ax *Apex) handleKeys() error {
	switch ax.os {
	case Darwin.String():
		publicKey := readKeyFile(darwinPublicKeyFile)
		privateKey := readKeyFile(darwinPrivateKeyFile)
		if publicKey != "" && privateKey != "" {
			ax.wireguardPubKey = publicKey
			ax.wireguardPvtKey = privateKey
			log.Infof("Existing key pair found at [ %s ] and [ %s ]", darwinPublicKeyFile, darwinPrivateKeyFile)
			return nil
		}
		log.Infof("No existing public/private key pair found, generating a new pair")
		if err := ax.generateKeyPair(darwinPublicKeyFile, darwinPrivateKeyFile); err != nil {
			log.Fatalf("Unable to locate or generate a key/pair %v", err)
		}
		log.Debugf("New keys were written to [ %s ] and [ %s ]", darwinPublicKeyFile, darwinPrivateKeyFile)
		return nil

	case Windows.String():
		publicKey := readKeyFile(windowsPublicKeyFile)
		privateKey := readKeyFile(windowsPrivateKeyFile)
		if publicKey != "" && privateKey != "" {
			ax.wireguardPubKey = publicKey
			ax.wireguardPvtKey = privateKey
			log.Infof("Existing key pair found at [ %s ] and [ %s ]", windowsPublicKeyFile, windowsPrivateKeyFile)
			return nil
		}
		log.Infof("No existing public/private key pair found, generating a new pair")
		if err := ax.generateKeyPair(windowsPublicKeyFile, windowsPrivateKeyFile); err != nil {
			log.Fatalf("Unable to locate or generate a key/pair %v", err)
		}
		log.Debugf("New keys were written to [ %s ] and [ %s ]", windowsPublicKeyFile, windowsPrivateKeyFile)
		return nil

	case Linux.String():
		publicKey := readKeyFile(linuxPublicKeyFile)
		privateKey := readKeyFile(linuxPrivateKeyFile)
		if publicKey != "" && privateKey != "" {
			ax.wireguardPubKey = publicKey
			ax.wireguardPvtKey = privateKey
			log.Infof("Existing key pair found at [ %s ] and [ %s ]", linuxPublicKeyFile, linuxPrivateKeyFile)
			return nil
		}
		log.Infof("No existing public/private key pair found, generating a new pair")
		if err := ax.generateKeyPair(linuxPublicKeyFile, linuxPrivateKeyFile); err != nil {
			log.Fatalf("Unable to locate or generate a key/pair %v", err)
		}
		log.Debugf("New keys were written to [ %s ] and [ %s ]", linuxPublicKeyFile, linuxPrivateKeyFile)
		return nil
	}
	return nil
}

// generateKeyPair a key pair and write them to disk
func (ax *Apex) generateKeyPair(publicKeyFile, privateKeyFile string) error {
	cmd := exec.Command(wgBinary, "genkey")
	privateKey, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("wg genkey error %s", err)
	}

	cmd = exec.Command(wgBinary, "pubkey")
	cmd.Stdin = bytes.NewReader(privateKey)
	publicKey, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("wg pubkey error: %s", err)
	}
	ax.wireguardPubKey = strings.TrimSpace(string(publicKey))
	ax.wireguardPvtKey = strings.TrimSpace(string(privateKey))

	// TODO remove this debug statement at some point
	log.Debugf("Public Key [ %s ] Private Key [ %s ]", ax.wireguardPubKey, ax.wireguardPvtKey)
	// write the new keys to disk
	writeToFile(ax.wireguardPubKey, publicKeyFile, publicKeyPermissions)
	writeToFile(ax.wireguardPvtKey, privateKeyFile, privateKeyPermissions)

	return nil
}

// readKeyFile reads the contents of a key file
func readKeyFile(keyFile string) string {
	if !FileExists(keyFile) {
		return ""
	}
	key, err := readKeyFileToString(keyFile)
	if err != nil {
		log.Tracef("unable to read key file: %v", err)
		return ""
	}

	return key
}

// readKeyFileToString reads the key file and strips any newline chars that create wireguard issues
func readKeyFileToString(s string) (string, error) {
	buf, err := os.ReadFile(s)
	if err != nil {
		return "", fmt.Errorf("unable to read file: %v\n", err)
	}
	rawStr := string(buf)
	return strings.Replace(rawStr, "\n", "", -1), nil
}
