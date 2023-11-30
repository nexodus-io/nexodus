package nexodus

import (
	"encoding/json"
	"fmt"
)

type ListPeersResponse struct {
	RelayPresent  bool                  `json:"relay-present"`
	RelayRequired bool                  `json:"relay-required"`
	Peers         map[string]WgSessions `json:"peers"`
}

func (ac *NexdCtl) ListPeers(_ string, result *string) error {
	peers, err := ac.nx.DumpPeersDefault()
	if err != nil {
		return fmt.Errorf("error getting list of peers: %w", err)
	}
	response := ListPeersResponse{
		Peers:         peers,
		RelayRequired: ac.nx.symmetricNat,
	}
	ac.nx.deviceCacheIterRead(func(d deviceCacheEntry) {
		if d.device.PublicKey == ac.nx.wireguardPubKey {
			return
		}
		p, ok := response.Peers[d.device.PublicKey]
		if !ok {
			return
		}
		p.Healthy = d.peerHealthy
		response.Peers[d.device.PublicKey] = p
		if d.peerHealthy && d.device.Relay {
			response.RelayPresent = true
		}
	})

	peersJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error marshalling list of peers: %w", err)
	}

	*result = string(peersJSON)

	return nil
}
