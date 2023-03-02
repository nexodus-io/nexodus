package util_test

import (
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoWithWaitGroup(t *testing.T) {

	// Verify you can use a nil WaitGroup
	util.GoWithWaitGroup(nil, func() {
	})

	// Verify all goroutines are done before the WaitGroup counter is zero.
	wg := &sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		// run a few functions in parallel
		util.GoWithWaitGroup(wg, func() {
			time.Sleep(200 * time.Millisecond)
		})
	}

	done := atomic.Bool{}
	go func() {
		wg.Wait()
		done.Store(true)
	}()

	// Wait should block about 200 ms
	assert.Eventually(t, func() bool {
		return done.Load()
	}, 400*time.Millisecond, time.Millisecond)

}
