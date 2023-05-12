package nexodus

import (
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"os"
	"runtime"
)

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and write them to location on the disk depending on the OS
func (ax *Nexodus) handleKeys() error {

	err := ax.stateStore.Load()
	if err != nil {
		return err
	}
	state := ax.stateStore.State()

	if state.PublicKey == "" || state.PrivateKey == "" {
		// We used to store the keys in a different location
		// migrate them to the state store
		state.PrivateKey, state.PublicKey, err = ax.loadLegacyKeys()
		if err != nil {
			return err
		}
		err = ax.stateStore.Store()
		if err != nil {
			return err
		}
	}

	if state.PublicKey != "" && state.PrivateKey != "" {
		ax.logger.Infof("Existing key pair found in [ %s ]", ax.stateStore)
	} else {
		ax.logger.Infof("No existing public/private key pair found, generating a new pair")
		wgKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate private key: %w", err)
		}
		state.PublicKey = wgKey.PublicKey().String()
		state.PrivateKey = wgKey.String()

		err = ax.stateStore.Store()
		if err != nil {
			return fmt.Errorf("failed store the keys: %w", err)
		}
		ax.logger.Debugf("New keys were written to [ %s ]", ax.stateStore)
	}

	ax.wireguardPubKey = state.PublicKey
	ax.wireguardPvtKey = state.PrivateKey
	return nil

}

// loadLegacyKeys should not be needed after everyone has upgraded to the latest nexd.
func (ax *Nexodus) loadLegacyKeys() (string, string, error) {

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

	// skip if the old key files don't exist
	if !(canReadFile(oldPubKeyFile) && canReadFile(oldPrivKeyFile)) {
		return "", "", nil
	}

	publicKey, err := os.ReadFile(oldPubKeyFile)
	if err != nil {
		return "", "", err
	}

	privateKey, err := os.ReadFile(oldPrivKeyFile)
	if err != nil {
		return "", "", err
	}

	return string(privateKey), string(publicKey), nil
}

func canReadFile(name string) bool {
	info, err := os.Stat(name)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}
