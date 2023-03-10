//go:build linux

package nexodus

import "fmt"

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and write them to location on the disk depending on the OS
func (ax *Nexodus) handleKeys() error {
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
