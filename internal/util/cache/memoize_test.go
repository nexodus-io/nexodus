package cache

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoizeCache_Memoize(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cache := NewMemoizeCache[string, string](time.Second * 1)

	counter := int32(0)
	pong := func() string {
		return fmt.Sprintf("pong %d", atomic.AddInt32(&counter, 1))
	}

	require.Equal("pong 1", cache.Memoize("ping", pong))
	// pong func should not be called since the first result should be cached for 1 sec
	require.Equal("pong 1", cache.Memoize("ping", pong))
	time.Sleep(1 * time.Second * 3 / 2) // wait til the cache expires...
	// pong func should get called again..
	require.Equal("pong 2", cache.Memoize("ping", pong))
}
