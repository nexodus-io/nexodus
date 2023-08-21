//go:build linux

package nexodus

import "fmt"

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and write them to location on the disk depending on the OS
func (ax *Nexodus) handleKeys() error {
	var pubKeyFile string
	var privKeyFile string
	if ax.userspaceMode {
		pubKeyFile = workdirPublicKeyFile
		privKeyFile = workdirPrivateKeyFile
	} else {
		pubKeyFile = linuxPublicKeyFile
		privKeyFile = linuxPrivateKeyFile
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
