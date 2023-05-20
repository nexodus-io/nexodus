package nexodus

import (
	"encoding/json"
	"fmt"
)

func (ac *NexdCtl) ListPeers(_ string, result *string) error {
	if ac.ax.userspaceMode {
		return fmt.Errorf("userspace mode not currently supported")
	}

	iface := ac.ax.defaultTunnelDev()
	if iface == "" {
		return fmt.Errorf("no tunnel interface found")
	}

	peers, err := DumpPeers(iface)
	if err != nil {
		return fmt.Errorf("error getting list of peers: %w", err)
	}

	peersJSON, err := json.Marshal(peers)
	if err != nil {
		return fmt.Errorf("error marshalling list of peers: %w", err)
	}

	*result = string(peersJSON)

	return nil
}
