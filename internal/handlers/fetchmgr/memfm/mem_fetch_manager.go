package memfm

import (
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/basefm"
	"gorm.io/gorm"
	"sync"
)

func New() fetchmgr.FetchManager {
	return basefm.New(func(key string, cacheSize int) basefm.Cache {
		return &cache{
			key:        key,
			ringBuffer: make([]fetchmgr.ResourceItem, cacheSize),
		}
	})
}

type cache struct {
	key        string
	mu         sync.RWMutex
	ringBuffer []fetchmgr.ResourceItem
	writePos   uint64
	// the number of open fetcher instances for this cache
	openCounter int
}

// cache implements the basefm.Cache interface
var _ basefm.Cache = &cache{}

func (cache *cache) OpenCounter() *int {
	return &cache.openCounter
}

func (cache *cache) Key() string {
	return cache.key
}

func (cache *cache) Fetch(fetcher *basefm.Fetcher, gtRevision uint64) (list fetchmgr.ResourceItemList, seq uint64) {
	items := fetchmgr.ResourceItemList{}

	// get items of the current cache...
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	writePos := cache.writePos
	ringSize := uint64(len(cache.ringBuffer))
	for fetcher.ReadPos != writePos {
		i := fetcher.ReadPos % ringSize
		item := cache.ringBuffer[i]
		fetcher.ReadPos += 1
		if gtRevision < item.Revision {
			if gtRevision+1 == item.Revision {
				items = append(items, item)
				gtRevision = item.Revision
			} else {
				//fmt.Println("sub.fetchFromTailCache = false")
				fetcher.FetchFromTailCache = false
				break
			}
		}
	}
	return items, writePos
}

func (cache *cache) Fill(fetcher *basefm.Fetcher, db *gorm.DB, gtRevision uint64, expectedWritePos uint64) (fetchmgr.ResourceList, error) {
	// multiple subs will compete to fill the cache... only the first in will do the work
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if expectedWritePos == cache.writePos {
		// fetch the data, and write it to the ring buffer...
		w, err := fetcher.FetchFn(db, gtRevision)
		if err != nil {
			return w, err
		}
		ringSize := uint64(len(cache.ringBuffer))

		fetchLength := w.Len()
		if fetchLength > int(ringSize) {
			// we fetched more than what fits in the ring, NOT ideal.
			fetchLength = int(ringSize)
		}

		for i := 0; i < fetchLength; i++ {
			item, revision, deletedAt := w.Item(i)
			cache.ringBuffer[cache.writePos%ringSize] = fetchmgr.ResourceItem{
				Item:      item,
				Revision:  revision,
				DeletedAt: deletedAt,
			}
			cache.writePos += 1
		}
		return w, nil

	}
	return nil, nil
}
