package util

import "sync"

// GoWithWaitGroup runs fn in a goroutine with an optional *sync.WaitGroup to
// track when fn finishes executing.
func GoWithWaitGroup(wg *sync.WaitGroup, fn func()) {
	if wg != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	} else {
		go fn()
	}
}
