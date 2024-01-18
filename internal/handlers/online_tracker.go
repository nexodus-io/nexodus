package handlers

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"sync"
	"time"
)

func New(logger *zap.Logger) (*DeviceTracker, error) {

	redisAddr := util.Getenv("NEXAPI_REDIS_SERVER", "redis:6379")
	redisDB, err := util.GetenvInt("NEXAPI_REDIS_DB", "1")
	if err != nil {
		return nil, err
	}

	return &DeviceTracker{
		redis: redis.NewClient(&redis.Options{
			Addr: redisAddr,
			DB:   redisDB,
		}),
		reconnectGracePeriod: time.Second * 5,
		logger:               logger,
		keyPrefix:            "dev-track:",
		localDevices:         map[string]int{},
	}, nil
}

// One of these exists off of the top-level API object.
type DeviceTracker struct {
	mu                   sync.Mutex
	redis                *redis.Client
	logger               *zap.Logger
	keyPrefix            string
	pubSub               *redis.PubSub
	localDevices         map[string]int
	reconnectGracePeriod time.Duration
}

func (ot *DeviceTracker) Connected(api *API, c *gin.Context, publicKey string, fn func()) {
	if publicKey == "" {
		fn()
		return
	}

	device := &models.Device{}
	result := api.DeviceIsOwnedByCurrentUser(c, api.db).First(&device, "public_key = ?", publicKey)
	if result.Error != nil {
		ot.logger.Warn("cannot track: invalid device public_key", zap.String("public_key", publicKey), zap.Error(result.Error))
		fn()
		return
	}

	if err := ot.connected(publicKey); err != nil {
		ot.logger.Warn("failed to update online redis state for device", zap.String("public_key", publicKey), zap.Error(err))
		fn()
		return
	}

	if !device.Online {
		device.Online = true
		now := time.Now()
		device.OnlineAt = &now
		err := api.db.Select("online", "online_at").Updates(device).Error
		if err != nil {
			ot.logger.Warn("failed to update db state for device", zap.String("public_key", publicKey), zap.Error(err))
			fn()
		}
	}

	defer func() {
		if err := ot.disconnected(publicKey); err != nil {
			ot.logger.Warn("failed to update offline redis state for device", zap.String("public_key", publicKey), zap.Error(err))
			return
		}
		time.AfterFunc(ot.reconnectGracePeriod, func() {
			connected, err := ot.isConnected(publicKey)
			if err != nil {
				ot.logger.Warn("failed to get online state for device", zap.String("public_key", publicKey), zap.Error(err))
				return
			}
			if connected {
				return
			}

			device := &models.Device{}
			result := api.db.First(&device, "public_key = ?", publicKey)
			if result.Error != nil {
				ot.logger.Warn("cannot track: invalid device public_key", zap.String("public_key", publicKey), zap.Error(result.Error))
				return
			}
			if device.Online {
				device.Online = false
				now := time.Now()
				device.OnlineAt = &now
				err := api.db.Select("online", "online_at").Updates(device).Error
				if err != nil {
					ot.logger.Warn("failed to update db state for device", zap.String("public_key", publicKey), zap.Error(err))
				}
			}
		})
	}()
	fn()
}

func (ot *DeviceTracker) connected(publicKey string) error {

	ot.mu.Lock()
	defer ot.mu.Unlock()

	// don't send a subscriptions to redis... it will only track 1 sub.
	if count, found := ot.localDevices[publicKey]; found {
		count++
		ot.localDevices[publicKey] = count
		return nil
	}

	ot.localDevices[publicKey] = 1
	if ot.pubSub == nil {
		ot.pubSub = ot.redis.Subscribe(context.Background(), ot.keyPrefix+publicKey)
	}

	err := ot.pubSub.Subscribe(context.Background(), ot.keyPrefix+publicKey)
	if err != nil {
		return err
	}
	return nil
}

func (ot *DeviceTracker) disconnected(publicKey string) error {

	ot.mu.Lock()
	defer ot.mu.Unlock()

	count, found := ot.localDevices[publicKey]
	if !found {
		return nil
	}

	count--
	if count != 0 {
		ot.localDevices[publicKey] = count
		return nil
	}

	delete(ot.localDevices, publicKey)
	err := ot.pubSub.Unsubscribe(context.Background(), ot.keyPrefix+publicKey)
	if err != nil {
		return err
	}
	return nil
}

func (ot *DeviceTracker) isConnected(publicKey string) (bool, error) {
	key := ot.keyPrefix + publicKey
	result, err := ot.redis.PubSubNumSub(context.Background(), key).Result()
	if err != nil {
		return false, err
	}
	return result[key] != 0, nil
}
