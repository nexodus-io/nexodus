package nexodus

import (
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// handleKeys will look for an existing key pair, if a pair is not found this method
// will generate a new pair and store them in the nexd persistent state
func (nx *Nexodus) handleKeys() error {

	err := nx.stateStore.Load()
	if err != nil {
		return err
	}
	state := nx.stateStore.State()

	if state.PublicKey != "" && state.PrivateKey != "" {
		nx.logger.Infof("Existing key pair found in [ %s ]", nx.stateStore)
	} else {
		nx.logger.Infof("No existing public/private key pair found, generating a new pair")
		wgKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate private key: %w", err)
		}
		state.PublicKey = wgKey.PublicKey().String()
		state.PrivateKey = wgKey.String()

		err = nx.stateStore.Store()
		if err != nil {
			return fmt.Errorf("failed store the keys: %w", err)
		}
		nx.logger.Debugf("New keys were written to [ %s ]", nx.stateStore)
	}

	nx.wireguardPubKey = state.PublicKey
	nx.wireguardPvtKey = state.PrivateKey
	return nil
}
