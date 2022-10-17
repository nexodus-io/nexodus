package controltower

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestInsertPeer(t *testing.T) {
	p := NewPeerMap()

	p1 := &Peer{
		ID:        uuid.New(),
		PublicKey: "1",
	}
	p.Insert(*p1)

	assert.Equal(t, p.cache[p1.ID], p1)
	assert.Contains(t, p.pubKeys["1"], p1.ID)
}

func TestInsertPeerSamePubKey(t *testing.T) {
	p := NewPeerMap()

	p1 := &Peer{
		ID:        uuid.New(),
		PublicKey: "1",
	}

	p2 := &Peer{
		ID:        uuid.New(),
		PublicKey: "1",
	}

	p.Insert(*p1)
	p.Insert(*p2)

	assert.Equal(t, p.cache[p1.ID], p1)
	assert.Equal(t, p.cache[p2.ID], p2)

	assert.Contains(t, p.pubKeys["1"], p1.ID)
	assert.Contains(t, p.pubKeys["1"], p2.ID)
}
