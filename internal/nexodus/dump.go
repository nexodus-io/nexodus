package nexodus

import (
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

// DumpPeers dump wireguard peers
func DumpPeers(iface string) ([]WgSessions, error) {
	c, err := wgctrl.New()
	if err != nil {
		return nil, err
	}
	device, err := c.Device(iface)
	if err != nil {
		return nil, err
	}
	peers := make([]WgSessions, 0)
	for _, peer := range device.Peers {
		peers = append(peers, WgSessions{
			PublicKey:       peer.PublicKey.String(),
			PreSharedKey:    peer.PresharedKey.String(),
			Endpoint:        peer.Endpoint.String(),
			AllowedIPs:      util.IPNetSliceToStringSlice(peer.AllowedIPs),
			LatestHandshake: peer.LastHandshakeTime.String(),
			Tx:              peer.TransmitBytes,
			Rx:              peer.ReceiveBytes,
		})
	}
	return peers, nil
}
