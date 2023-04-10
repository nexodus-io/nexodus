package nexodus

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net"
)

const batchSize = 10

func (ac *NexdCtl) Connectivity(_ string, keepaliveResults *string) error {
	res := ac.ax.connectivityProbe()
	var err error

	// Marshal the map into a JSON string.
	keepaliveJson, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("error marshalling connectivty results")
	}

	*keepaliveResults = string(keepaliveJson)

	return nil
}

func (ax *Nexodus) connectivityProbe() map[string]KeepaliveStatus {
	if ax.userspaceMode {
		// TODO: to be addressed in #703 'nexd proxy: Add support for nexd keepalives'
		ax.logger.Warn("nexctl connection checks are currently unsupported for nexd user space proxy")
		return make(map[string]KeepaliveStatus)
	}

	peerStatusMap := make(map[string]KeepaliveStatus)

	if !ax.relay {
		for _, value := range ax.deviceCache {
			// skip the node sourcing the probe
			if ax.wireguardPubKey == value.PublicKey {
				continue
			}
			pubKey := value.PublicKey
			nodeAddr := value.TunnelIp
			if net.ParseIP(value.TunnelIp) == nil {
				ax.logger.Debugf("failed parsing an ip from the prefix %s", value.TunnelIp)
				continue
			}
			hostname := value.Hostname
			peerStatusMap[pubKey] = KeepaliveStatus{
				WgIP:        nodeAddr,
				IsReachable: false,
				Hostname:    hostname,
			}
		}
	}
	connResults := probeConnectivity(peerStatusMap, ax.logger)

	return connResults
}

// probeConnectivity check connectivity in batches to limit excessive traffic in the case of a large number of peers
func probeConnectivity(peers map[string]KeepaliveStatus, logger *zap.SugaredLogger) map[string]KeepaliveStatus {
	peerConnResultsMap := make(map[string]KeepaliveStatus)

	peerKeys := make([]string, 0, len(peers))
	for key := range peers {
		peerKeys = append(peerKeys, key)
	}

	for i := 0; i < len(peerKeys); i += batchSize {
		end := i + batchSize
		if end > len(peerKeys) {
			end = len(peerKeys)
		}

		batch := peerKeys[i:end]

		c := make(chan struct {
			KeepaliveStatus
			IsReachable bool
		})

		for _, pubKey := range batch {
			go runProbe(peers[pubKey], c)
		}

		for range batch {
			result := <-c
			ip := result.WgIP

			if result.IsReachable {
				logger.Debugf("connectivty probe [ %s ] is reachable", ip)
			} else {
				logger.Debugf("connectivty probe [ %s ] is not reachable", ip)
			}

			peerConnResultsMap[ip] = KeepaliveStatus{
				WgIP:        result.WgIP,
				IsReachable: result.IsReachable,
				Hostname:    result.Hostname,
			}
		}
	}

	return peerConnResultsMap
}
