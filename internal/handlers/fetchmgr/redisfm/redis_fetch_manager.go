package redisfm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/basefm"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"sync"
	"time"
)

type options struct {
	redis        *redis.Client
	fetchTimeout time.Duration
	logger       *zap.Logger
	keyPrefix    string
}

func New(redis *redis.Client, fetchTimeout time.Duration, logger *zap.Logger, keyPrefix string) fetchmgr.FetchManager {
	options := &options{
		redis:        redis,
		fetchTimeout: fetchTimeout,
		logger:       logger,
		keyPrefix:    keyPrefix,
	}
	return basefm.New(func(key string, cacheSize int) basefm.Cache {
		return &cache{
			options:   options,
			key:       key,
			cacheSize: int64(cacheSize),
		}
	})
}

type cache struct {
	*options
	key       string
	mu        sync.RWMutex
	cacheSize int64
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

func (cache *cache) Fetch(f *basefm.Fetcher, lastRevision uint64) (fetchmgr.ResourceItemList, uint64) {
	items := fetchmgr.ResourceItemList{}

	// get items of the current cache...
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	redisClient := cache.redis

	ctx, cancel := context.WithTimeout(context.Background(), cache.fetchTimeout)
	defer cancel()

	redisKey := cache.keyPrefix + cache.key
	streamList, err := redisClient.XRead(ctx, &redis.XReadArgs{
		Streams: []string{redisKey, fmt.Sprintf("0-%d", lastRevision)},
		Block:   -1,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return items, 0
		}
		cache.options.logger.Warn("Fetch XREAD failed", zap.Error(err))
		f.FetchFromTailCache = false
		return items, 0
	}

	for _, r := range streamList[0].Messages {
		item := fetchmgr.ResourceItem{}

		err = json.Unmarshal([]byte(r.Values["m"].(string)), &item)
		if err != nil {
			cache.options.logger.Warn("fetchFromRing item marshal failed", zap.Error(err))
			f.FetchFromTailCache = false
			return items, 0
		}
		if lastRevision < item.Revision {
			if lastRevision+1 == item.Revision {
				items = append(items, item)
				lastRevision = item.Revision
			} else {
				f.FetchFromTailCache = false
				break
			}
		}
	}
	return items, 0
}

func (cache *cache) Fill(f *basefm.Fetcher, db *gorm.DB, gtRevision uint64, seq uint64) (fetchmgr.ResourceList, error) {
	// only allow one fetcher to fill the cache at a time to avoid
	cache.mu.Lock()
	defer cache.mu.Unlock()

	redisClient := cache.redis
	ctx, cancel := context.WithTimeout(context.Background(), cache.fetchTimeout)
	defer cancel()

	redisKey := cache.keyPrefix + cache.key
	messages, err := redisClient.XRangeN(ctx, redisKey, fmt.Sprintf("0-%d", gtRevision+1), "+", 1).Result()
	if err != nil {
		cache.options.logger.Warn("Fill XRangeN failed", zap.Error(err))
		return nil, nil
	}
	if len(messages) != 0 {
		return nil, nil
	}

	// fetch the data, and write it to the ring buffer...
	w, err := f.FetchFn(db, gtRevision)
	if err != nil {
		return w, err
	}
	fetchLength := w.Len()
	if fetchLength > int(cache.cacheSize) {
		// we fetched more than what fits in the ring, NOT ideal.
		fetchLength = int(cache.cacheSize)
	}

	addCmds, err := redisClient.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for i := 0; i < fetchLength; i++ {
			item, revision, deletedAt := w.Item(i)
			data, err := json.Marshal(fetchmgr.ResourceItem{
				Item:      item,
				Revision:  revision,
				DeletedAt: deletedAt,
			})
			if err != nil {
				return err
			}

			pipe.XAdd(ctx, &redis.XAddArgs{
				Stream: redisKey,
				MaxLen: cache.cacheSize,
				ID:     fmt.Sprintf("0-%d", revision),
				Values: map[string]interface{}{
					"m": string(data),
				},
			})
		}
		return nil
	})
	if err != nil {
		// this error is normal, it just means another replica was able to fill the cache before we could.
		if err.Error() != "ERR The ID specified in XADD is equal or smaller than the target stream top item" {
			cache.options.logger.Warn("Fill XADD failed", zap.Error(err), zap.String("cmds", fmt.Sprintf("%+v", addCmds)))
		}
		return nil, nil
	}
	return w, nil
}
