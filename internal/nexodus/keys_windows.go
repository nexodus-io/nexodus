//go:build windows

package nexodus

import "fmt"

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and write them to location on the disk depending on the OS
func (nx *Nexodus) handleKeys() error {
	var pubKeyFile string
	var privKeyFile string
	if nx.userspaceMode {
		pubKeyFile = workdirPublicKeyFile
		privKeyFile = workdirPrivateKeyFile
	} else {
		pubKeyFile = windowsPublicKeyFile
		privKeyFile = windowsPrivateKeyFile
	}
	publicKey := readKeyFile(nx.logger, pubKeyFile)
	privateKey := readKeyFile(nx.logger, privKeyFile)
	if publicKey != "" && privateKey != "" {
		nx.wireguardPubKey = publicKey
		nx.wireguardPvtKey = privateKey
		nx.logger.Infof("Existing key pair found at [ %s ] and [ %s ]", pubKeyFile, privKeyFile)
		return nil
	}
	nx.logger.Infof("No existing public/private key pair found, generating a new pair")
	if err := nx.generateKeyPair(pubKeyFile, privKeyFile); err != nil {
		return fmt.Errorf("Unable to locate or generate a key/pair: %w", err)
	}
	nx.logger.Debugf("New keys were written to [ %s ] and [ %s ]", pubKeyFile, privKeyFile)
	return nil
}
