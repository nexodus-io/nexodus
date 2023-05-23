package nexodus

import (
	"strings"

	"github.com/nexodus-io/nexodus/internal/util"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// WgSessions wireguard peer session information
type WgSessions struct {
	PublicKey       string
	PreSharedKey    string
	Endpoint        string
	AllowedIPs      []string
	LatestHandshake string
	Tx              int64
	Rx              int64
}

func (nx *Nexodus) DumpPeers(iface string) (map[string]WgSessions, error) {
	if nx.userspaceMode {
		return nx.DumpPeersUS(iface)
	}
	return DumpPeersOS(iface)
}

func (nx *Nexodus) DumpPeersUS(iface string) (map[string]WgSessions, error) {
	fullConfig, err := nx.userspaceDev.IpcGet()
	if err != nil {
		nx.logger.Errorf("Failed to read back full wireguard config: %w", err)
		return nil, err
	}

	// Every new peer starts with public_key
	//
	// public_key=73e7b02320b07e3566b1064443a65f76191a89430cf479cb9d17d8926087d04a
	// preshared_key=0000000000000000000000000000000000000000000000000000000000000000
	// protocol_version=1
	// endpoint=172.17.0.2:51815
	// last_handshake_time_sec=0
	// last_handshake_time_nsec=0
	// tx_bytes=148
	// rx_bytes=0
	// persistent_keepalive_interval=20
	// allowed_ip=100.100.0.2/32
	// allowed_ip=200::2/128

	// Parse fullConfig string by line of key=value pairs
	// and build a list of WgSessions
	peers := make(map[string]WgSessions)
	peer := WgSessions{}
	for _, line := range strings.Split(fullConfig, "\n") {
		kv := util.SplitKeyValue(line)
		if kv == nil || len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "public_key":
			if peer.PublicKey != "" {
				// Add previous peer
				peers[peer.PublicKey] = peer
				peer = WgSessions{}
			}
			peer.PublicKey = kv[1]
		case "preshared_key":
			peer.PreSharedKey = kv[1]
		case "endpoint":
			peer.Endpoint = kv[1]
		case "last_handshake_time_sec":
			peer.LatestHandshake = kv[1]
		case "tx_bytes":
			peer.Tx = util.StringToInt64(kv[1])
		case "rx_bytes":
			peer.Rx = util.StringToInt64(kv[1])
		case "allowed_ip":
			peer.AllowedIPs = append(peer.AllowedIPs, kv[1])
		}
	}
	// Add last peer
	if peer.PublicKey != "" {
		peers[peer.PublicKey] = peer
	}
	return peers, nil
}

// DumpPeers dump wireguard peers
func DumpPeersOS(iface string) (map[string]WgSessions, error) {
	c, err := wgctrl.New()
	if err != nil {
		return nil, err
	}
	device, err := c.Device(iface)
	if err != nil {
		return nil, err
	}
	peers := make(map[string]WgSessions)
	for _, peer := range device.Peers {
		pubKey := peer.PublicKey.String()
		peers[pubKey] = WgSessions{
			PublicKey:       pubKey,
			PreSharedKey:    peer.PresharedKey.String(),
			Endpoint:        peer.Endpoint.String(),
			AllowedIPs:      util.IPNetSliceToStringSlice(peer.AllowedIPs),
			LatestHandshake: peer.LastHandshakeTime.String(),
			Tx:              peer.TransmitBytes,
			Rx:              peer.ReceiveBytes,
		}
	}
	return peers, nil
}
