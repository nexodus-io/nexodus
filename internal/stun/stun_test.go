package stun

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextStunServer(t *testing.T) {
	assert := assert.New(t)

	serverCounts := make(map[string]int)

	// Call NextStunServer len(stunServers)*2 times to ensure that we cycle through the servers at least once.
	for i := 0; i < len(stunServers)*2; i++ {
		server := NextServer()

		// Assert that a non-empty string is returned
		assert.NotEmpty(server)

		serverCounts[server]++
	}

	// Assert that all servers are returned at least once
	for _, server := range stunServers {
		count, exists := serverCounts[server]
		assert.True(exists, "Server was not returned by NextStunServer: %s", server)
		assert.GreaterOrEqual(count, 1, "Server was returned less than once: %s", server)
	}
}
