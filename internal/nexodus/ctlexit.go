package nexodus

import (
	"encoding/json"
	"fmt"
)

func (ac *NexdCtl) EnableExitNodeClient(_ string, result *string) error {
	err := ac.nx.ExitNodeClientSetup()

	enableExitNodeClientJson, err := json.Marshal(err)
	if err != nil {
		return fmt.Errorf("error marshalling enable exit node results %w", err)
	}

	*result = string(enableExitNodeClientJson)

	return nil
}

func (ac *NexdCtl) DisableExitNodeClient(_ string, result *string) error {
	err := ac.nx.exitNodeClientTeardown()

	disableExitNodeClientJson, err := json.Marshal(err)
	if err != nil {
		return fmt.Errorf("error marshalling disable exit node client results %w", err)
	}

	*result = string(disableExitNodeClientJson)

	return nil
}

// ListExitNodes lists all exit node origins
func (ac *NexdCtl) ListExitNodes(_ string, result *string) error {
	var allExitNodeOrigins []wgPeerConfig
	isExitNode := false

	// Check if the local node is an exit node
	for _, prefix := range ac.nx.childPrefix {
		if prefix == "0.0.0.0/0" {
			isExitNode = true
			break
		}
	}

	// If the local node is an exit node, create a copy and append the new instance
	if isExitNode {
		// Make a copy of exitNodeOrigins to a new slice
		allExitNodeOrigins = make([]wgPeerConfig, len(ac.nx.exitNode.exitNodeOrigins))
		copy(allExitNodeOrigins, ac.nx.exitNode.exitNodeOrigins)

		// Create a new instance of wgPeerConfig
		newPeerConfig := wgPeerConfig{
			PublicKey: ac.nx.wireguardPubKey,
			Endpoint:  ac.nx.nodeReflexiveAddressIPv4.String(),
		}
		// Append the local node if it is an exit node
		allExitNodeOrigins = append(allExitNodeOrigins, newPeerConfig)
	} else {
		allExitNodeOrigins = ac.nx.exitNode.exitNodeOrigins
	}

	exitNodeOriginsJSON, err := json.Marshal(allExitNodeOrigins)
	if err != nil {
		return fmt.Errorf("error marshalling exit node list results: %w", err)
	}

	*result = string(exitNodeOriginsJSON)

	return nil
}
