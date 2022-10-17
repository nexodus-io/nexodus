package controltower

import (
	"encoding/json"

	"github.com/google/uuid"
)

type PubKey struct {
	ID    string
	Peers map[uuid.UUID]struct{}
}

func NewPubKey(key string) *PubKey {
	return &PubKey{
		ID:    key,
		Peers: make(map[uuid.UUID]struct{}),
	}
}

func (p *PubKey) MarshalJSON() ([]byte, error) {
	peers := make([]uuid.UUID, 0)
	for k := range p.Peers {
		peers = append(peers, k)
	}
	return json.Marshal(
		struct {
			ID    string      `json:"id"`
			Peers []uuid.UUID `json:"peers"`
		}{
			ID:    p.ID,
			Peers: peers,
		})
}
