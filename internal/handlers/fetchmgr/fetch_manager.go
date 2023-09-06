package fetchmgr

import (
	"gorm.io/gorm"
)

// One of these exists off of the top-level API object.`
type FetchFn func(db *gorm.DB, gtRevision uint64) (ResourceList, error)

func (f FetchFn) Fetch(db *gorm.DB, gtRevision uint64) (ResourceList, error) {
	return f(db, gtRevision)
}

func (f FetchFn) Close() {
}

// One of these exists off of the top-level API object.
type FetchManager interface {
	Open(key string, cacheSize int, fetchFn FetchFn) Fetcher
}

type Fetcher interface {
	Fetch(db *gorm.DB, gtRevision uint64) (ResourceList, error)
	Close()
}
