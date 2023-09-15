package nonefm

import (
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
)

func New() fetchmgr.FetchManager {
	return &none{}
}

type none struct{}

func (n none) Open(key string, cacheSize int, fetchFn fetchmgr.FetchFn) fetchmgr.Fetcher {
	return fetchFn
}
