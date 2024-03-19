package nexodus

import (
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api"
	"net"

	"go.uber.org/zap"
)

const (
	batchSize = 10
	v4        = "v4"
	v6        = "v6"
)

// ConnectivityV4 pings all peers via IPv4
func (ac *NexdCtl) ConnectivityV4(_ string, keepaliveResults *string) error {
	res := ac.nx.connectivityProbe(v4)
	var err error

	// Marshal the map into a JSON string.
	keepaliveJson, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("error marshalling connectivty results")
	}

	*keepaliveResults = string(keepaliveJson)

	return nil
}

// ConnectivityV6 pings all peers via IPv6
func (ac *NexdCtl) ConnectivityV6(_ string, keepaliveResults *string) error {
	res := ac.nx.connectivityProbe(v6)
	var err error

	// Marshal the map into a JSON string.
	keepaliveJson, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("error marshalling connectivty results")
	}

	*keepaliveResults = string(keepaliveJson)

	return nil
}

func (nx *Nexodus) connectivityProbe(family string) api.PingPeersResponse {
	peersByKey := make(map[string]api.KeepaliveStatus)
	res := api.PingPeersResponse{
		RelayRequired: nx.symmetricNat,
	}
	if !nx.relay {
		nx.deviceCacheIterRead(func(value deviceCacheEntry) {
			// skip the node sourcing the probe
			if nx.wireguardPubKey == value.device.GetPublicKey() {
				return
			}
			var nodeAddr string
			pubKey := value.device.GetPublicKey()
			if family == v6 {
				nodeAddr = value.device.Ipv6TunnelIps[0].GetAddress()
			} else {
				nodeAddr = value.device.Ipv4TunnelIps[0].GetAddress()
			}
			if net.ParseIP(nodeAddr) == nil {
				nx.logger.Debugf("failed parsing an ip address from %s", nodeAddr)
				return
			}

			hostname := value.device.GetHostname()
			peersByKey[pubKey] = api.KeepaliveStatus{
				WgIP:        nodeAddr,
				IsReachable: false,
				Hostname:    hostname,
				Method:      value.peeringMethod,
			}
		})
	}
	res.Peers = nx.probeConnectivity(peersByKey, nx.logger)

	return res
}

// probeConnectivity check connectivity in batches to limit excessive traffic in the case of a large number of peers
func (nx *Nexodus) probeConnectivity(peersByKey map[string]api.KeepaliveStatus, logger *zap.SugaredLogger) map[string]api.KeepaliveStatus {
	peerConnResultsMap := make(map[string]api.KeepaliveStatus)

	peerKeys := make([]string, 0, len(peersByKey))
	for key := range peersByKey {
		peerKeys = append(peerKeys, key)
	}

	for i := 0; i < len(peerKeys); i += batchSize {
		end := i + batchSize
		if end > len(peerKeys) {
			end = len(peerKeys)
		}

		batch := peerKeys[i:end]

		c := make(chan struct {
			api.KeepaliveStatus
			IsReachable bool
		})

		for _, pubKey := range batch {
			go nx.runProbe(peersByKey[pubKey], c)
		}

		for range batch {
			result := <-c
			ip := result.WgIP

			if result.IsReachable {
				logger.Debugf("connectivty probe [ %s ] is reachable", ip)
			} else {
				logger.Debugf("connectivty probe [ %s ] is not reachable", ip)
			}

			peerConnResultsMap[ip] = api.KeepaliveStatus{
				WgIP:        result.WgIP,
				IsReachable: result.IsReachable,
				Hostname:    result.Hostname,
				Latency:     result.Latency,
				Method:      result.Method,
			}
		}
	}

	return peerConnResultsMap
}
