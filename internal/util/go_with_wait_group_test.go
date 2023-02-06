package util_test

import (
	"github.com/redhat-et/apex/internal/util"
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
	counter := atomic.Int32{}
	wg := &sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		// run a few functions in parallel
		util.GoWithWaitGroup(wg, func() {
			time.Sleep(200 * time.Millisecond)
			counter.Add(1)
		})
	}
	wg.Wait()
	assert.Equal(t, int32(10), counter.Load())

}
