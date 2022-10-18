package controltower

import (
	"github.com/google/uuid"
)

// Peer represents data about a Peer's record.
type Peer struct {
	ID          uuid.UUID `json:"id"`
	PublicKey   string    `json:"public-key"`
	EndpointIP  string    `json:"endpoint-ip"`
	AllowedIPs  string    `json:"allowed-ips"`
	Zone        string    `json:"zone"`
	NodeAddress string    `json:"node-address"`
	ChildPrefix string    `json:"child-prefix"`
	HubRouter   bool      `json:"hub-router"`
	HubZone     bool      `json:"hub-zone"`
	ZonePrefix  string    `json:"zone-prefix"`
}

func NewPeer(publicKey, endpointIP, allowedIPs, zone, nodeAddress, childPrefix string) Peer {
	return Peer{
		ID:          uuid.New(),
		PublicKey:   publicKey,
		EndpointIP:  endpointIP,
		AllowedIPs:  allowedIPs,
		Zone:        zone,
		NodeAddress: nodeAddress,
		ChildPrefix: childPrefix,
	}
}
