package basefm

import (
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"gorm.io/gorm"
	"sync"
)

// One of these exists off of the top-level API object.
type CacheBasedFetchManager struct {
	mu sync.Mutex
	// The key for a Cache can be anything, but it should uniquely identify
	// a list of resources that would be repeatedly queried, such as the list of all
	// devices in an organization. In that case, the key could be `org-devices:<ORG-UUID>`.
	caches     map[string]Cache
	newCacheFn func(key string, cacheSize int) Cache
}

// CacheBasedFetchManager implements the fetchmgr.FetchManager interface
var _ fetchmgr.FetchManager = &CacheBasedFetchManager{}

func New(newCacheFn func(key string, cacheSize int) Cache) fetchmgr.FetchManager {
	return &CacheBasedFetchManager{
		caches:     map[string]Cache{},
		newCacheFn: newCacheFn,
	}
}

type Cache interface {
	OpenCounter() *int
	Key() string
	Fetch(fetcher *Fetcher, gtRevision uint64) (list fetchmgr.ResourceItemList, seq uint64)
	Fill(fetcher *Fetcher, db *gorm.DB, gtRevision uint64, seq uint64) (fetchmgr.ResourceList, error)
}

func (manager *CacheBasedFetchManager) Open(key string, cacheSize int, fetchFn fetchmgr.FetchFn) fetchmgr.Fetcher {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	cache := manager.caches[key]
	if cache == nil {
		cache = manager.newCacheFn(key, cacheSize)
	}
	manager.caches[key] = cache
	*cache.OpenCounter() += 1
	return &Fetcher{
		manager:            manager,
		cache:              cache,
		FetchFn:            fetchFn,
		FetchFromTailCache: false,
	}
}

// Created by CacheBasedFetchManager.Open()
// The same Cache key can be opened multiple times. Each client that uses the same cache has
// its own instance of Fetcher, and they all point to the same Cache.
type Fetcher struct {
	manager            *CacheBasedFetchManager
	cache              Cache
	FetchFn            func(db *gorm.DB, gtRevision uint64) (fetchmgr.ResourceList, error)
	FetchFromTailCache bool
	ReadPos            uint64
}

func (f *Fetcher) Close() {
	if f.cache == nil {
		return
	}
	f.manager.mu.Lock()
	defer f.manager.mu.Unlock()
	*f.cache.OpenCounter() -= 1
	if *f.cache.OpenCounter() == 0 {
		delete(f.manager.caches, f.cache.Key())
	}
	f.cache = nil
}

func (f *Fetcher) Fetch(db *gorm.DB, gtRevision uint64) (fetchmgr.ResourceList, error) {
	for f.FetchFromTailCache {
		items, writePos := f.cache.Fetch(f, gtRevision)
		if len(items) != 0 {
			return items, nil
		}

		// did we fall behind the tail cache?
		if !f.FetchFromTailCache {
			break
		}

		// Try to fill the tail cache...
		result, err := f.cache.Fill(f, db, gtRevision, writePos)
		if result != nil || err != nil {
			return result, err
		}

		// if we get here, we were not the Fetcher that filled the ring, loop around
		// to see if we can use the results of the other Fetcher that did do the work.
	}

	// this Fetcher has not caught up with the tail cache, it's still fetching from the DB
	wl, err := f.FetchFn(db, gtRevision)
	if err != nil {
		return wl, err
	}

	// empty fetch means we have hit the tail... next fetch will load from the tail cache
	if wl.Len() == 0 {
		f.FetchFromTailCache = true
	}
	return wl, nil
}
