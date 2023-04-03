//go:build darwin

package wireguard

import "fmt"

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and write them to location on the disk depending on the OS
func (wg *WireGuard) HandleKeys() error {
	var pubKeyFile string
	var privKeyFile string
	if wg.UserspaceMode {
		pubKeyFile = workdirPublicKeyFile
		privKeyFile = workdirPrivateKeyFile
	} else {
		pubKeyFile = darwinPublicKeyFile
		privKeyFile = darwinPrivateKeyFile
	}

	publicKey := readKeyFile(wg.Logger, pubKeyFile)
	privateKey := readKeyFile(wg.Logger, privKeyFile)
	if publicKey != "" && privateKey != "" {
		wg.WireguardPubKey = publicKey
		wg.WireguardPvtKey = privateKey
		wg.Logger.Infof("Existing key pair found at [ %s ] and [ %s ]", pubKeyFile, privKeyFile)
		return nil
	}
	wg.Logger.Infof("No existing public/private key pair found, generating a new pair")
	if err := wg.generateKeyPair(pubKeyFile, privKeyFile); err != nil {
		return fmt.Errorf("Unable to locate or generate a key/pair: %w", err)
	}
	wg.Logger.Debugf("New keys were written to [ %s ] and [ %s ]", pubKeyFile, privKeyFile)
	return nil

}
