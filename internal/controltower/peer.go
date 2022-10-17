package controltower

import (
	"fmt"

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

type PeerMap struct {
	cache   map[uuid.UUID]*Peer
	pubKeys map[string]map[uuid.UUID]struct{}
}

func NewPeerMap() *PeerMap {
	return &PeerMap{
		cache:   make(map[uuid.UUID]*Peer),
		pubKeys: make(map[string]map[uuid.UUID]struct{}),
	}
}

func (m *PeerMap) Insert(p Peer) {
	m.cache[p.ID] = &p
	if _, ok := m.pubKeys[p.PublicKey]; !ok {
		m.pubKeys[p.PublicKey] = make(map[uuid.UUID]struct{})
	}
	m.pubKeys[p.PublicKey][p.ID] = struct{}{}
}

func (m *PeerMap) List() []*Peer {
	res := make([]*Peer, 0)
	for _, v := range m.cache {
		res = append(res, v)
	}
	return res
}

func (m *PeerMap) ListByPubKey(key string) []*Peer {
	res := make([]*Peer, 0)
	if v, ok := m.pubKeys[key]; ok {
		for pid := range v {
			res = append(res, m.cache[pid])
		}
	}
	return res
}

func (m *PeerMap) Get(id uuid.UUID) (*Peer, error) {
	if peer, ok := m.cache[id]; ok {
		return peer, nil
	}
	return nil, fmt.Errorf("peer not found")
}
