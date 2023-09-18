package tests

import (
	"fmt"
	. "github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	// This test depends on increased time resolution,
	// the following call turns on for windows.
	_ = util.TimeBeginPeriod(1)
}

func TestFetchManagerReducesDBFetchesAtTheTail(t *testing.T, managers ...FetchManager) {
	require := require.New(t)

	sourceVersion := int32(0)
	sourceHits := int32(0)
	source := func(db *gorm.DB, gtRevision uint64) (ResourceList, error) {
		atomic.AddInt32(&sourceHits, 1)
		time.Sleep(5 * time.Millisecond)
		result := ResourceItemList{}
		version := int(atomic.LoadInt32(&sourceVersion))
		switch version {
		case 0:
			if gtRevision < 20 {
				revision := gtRevision + uint64(1)
				result = append(result, ResourceItem{Item: fmt.Sprintf("item%d", revision), Revision: revision})
			}
		case 1:
			revision := gtRevision + uint64(1)
			result = append(result, ResourceItem{Item: fmt.Sprintf("item%d", revision), Revision: revision})
		}
		return result, nil
	}

	waitPoint1 := sync.WaitGroup{}
	waitPoint1.Add(1)
	waitPoint2 := sync.WaitGroup{}
	waitPoint2.Add(1)

	stepCounter := int32(0)
	fetcherCount := 50

	waitForAllWorkersToCompleteNextStep := func(waitFor time.Duration) {
		require.Eventuallyf(func() bool {
			return atomic.LoadInt32(&stepCounter) == int32(fetcherCount)
		}, waitFor, 1*time.Millisecond, "Not enough fetchers finished in time: %d", atomic.LoadInt32(&stepCounter))
		atomic.StoreInt32(&stepCounter, 0)
	}

	for i := 0; i < fetcherCount; i++ {
		go func(fetcherId int) {
			fetcher := managers[fetcherId%len(managers)].Open("key1", 10, source)
			defer fetcher.Close()
			lastSeq := uint64(0)
			for {
				wl, err := fetcher.Fetch(nil, lastSeq)
				require.NoError(err)
				if wl.Len() == 0 {
					break
				}
				for i := 0; i < wl.Len(); i++ {
					_, seq, _ := wl.Item(i)
					require.Equal(lastSeq+1, seq)
					lastSeq = seq
				}
			}
			require.Equal(20, int(lastSeq))

			atomic.AddInt32(&stepCounter, 1)
			waitPoint1.Wait()

			tillSeq := lastSeq + 20
			for lastSeq < tillSeq {
				wl, err := fetcher.Fetch(nil, lastSeq)
				require.NoError(err)
				for i := 0; i < wl.Len(); i++ {
					_, seq, _ := wl.Item(i)
					require.Equal(lastSeq+1, seq)
					lastSeq = seq
				}
			}
			atomic.AddInt32(&stepCounter, 1)
			waitPoint2.Wait()

			tillSeq = lastSeq + 20
			stallSeq := lastSeq
			for lastSeq < tillSeq {
				// simulate 5 of our fetchers being slow, so that they can't keep up with the tail cache
				if fetcherId < 5 && lastSeq == stallSeq {
					time.Sleep(2 * time.Second)
				}
				wl, err := fetcher.Fetch(nil, lastSeq)
				require.NoError(err)
				for i := 0; i < wl.Len(); i++ {
					_, seq, _ := wl.Item(i)
					require.Equal(lastSeq+1, seq)
					lastSeq = seq
				}
			}
			atomic.AddInt32(&stepCounter, 1)

		}(i)
	}

	waitForAllWorkersToCompleteNextStep(1 * time.Second)

	// All the initial fetch hit the source. 21*50 = 1050
	require.Equal(1050, int(atomic.LoadInt32(&sourceHits)))
	atomic.StoreInt32(&sourceHits, 0)

	// switch to the next source impl..
	atomic.StoreInt32(&sourceVersion, 1)
	waitPoint1.Done()

	// wait for the next round of fetches to complete...
	waitForAllWorkersToCompleteNextStep(5 * time.Second)
	// now that the fetchers are reading from the tail cache, the source should only be hit a few times.
	require.Less(int(atomic.LoadInt32(&sourceHits)), 120)
	atomic.StoreInt32(&sourceHits, 0)
	waitPoint2.Done()

	// 5 fetchers will fall behind and not be able to read from the tail cache, so
	// this should cause additional source hits.
	waitForAllWorkersToCompleteNextStep(5 * time.Second)
	require.GreaterOrEqual(int(atomic.LoadInt32(&sourceHits)), 120)
	require.Less(int(atomic.LoadInt32(&sourceHits)), 500)
	atomic.StoreInt32(&sourceHits, 0)

}
