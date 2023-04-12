package signalbus

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func notifyAfter(bus *signalBus, name string, d time.Duration) {
	go func() {
		time.Sleep(d)
		bus.Notify(name)
	}()
}

func TestNewSignalBus(t *testing.T) {
	require := require.New(t)

	bus := NewSignalBus().(*signalBus)

	// it's ok to send notifications before subscriptions...
	bus.Notify("unknown")

	aSub1 := bus.Subscribe("a")

	// in about a second the subscription should get signaled..
	notifyAfter(bus, "a", 1*time.Second)
	require.False(aSub1.IsSignaled())
	require.Eventually(aSub1.IsSignaled, 2*time.Second, time.Millisecond)

	// Verify that the same subs share memory structs...
	require.Equal(1, len(bus.signals))
	aSub2 := bus.Subscribe("a")
	require.Equal(1, len(bus.signals))

	// Verify that notifications work on both subs...
	notifyAfter(bus, "a", 1*time.Second)
	require.False(aSub1.IsSignaled())
	require.False(aSub2.IsSignaled())
	require.Eventually(aSub1.IsSignaled, 2*time.Second, time.Millisecond)
	require.Eventually(aSub2.IsSignaled, 2*time.Second, time.Millisecond)

	// Closing all the subs to the same named signal will release memory..
	aSub1.Close()
	require.Equal(1, len(bus.signals))
	aSub2.Close()
	require.Equal(0, len(bus.signals))
}
