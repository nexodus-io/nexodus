package handlers

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"sync"
	"time"
)

func New(logger *zap.Logger) (*AgentTracker, error) {

	redisAddr := util.Getenv("NEXAPI_REDIS_SERVER", "redis:6379")
	redisDB, err := util.GetenvInt("NEXAPI_REDIS_DB", "1")
	if err != nil {
		return nil, err
	}

	return &AgentTracker{
		redis: redis.NewClient(&redis.Options{
			Addr:             redisAddr,
			DB:               redisDB,
			DisableIndentity: true,
		}),
		reconnectGracePeriod: time.Second * 5,
		logger:               logger,
		keyPrefix:            "agent-track:",
		localAgents:          map[string]int{},
	}, nil
}

// One of these exists off of the top-level API object.
type AgentTracker struct {
	mu                   sync.Mutex
	redis                *redis.Client
	logger               *zap.Logger
	keyPrefix            string
	pubSub               *redis.PubSub
	localAgents          map[string]int
	reconnectGracePeriod time.Duration
}

func (at *AgentTracker) Connected(api *API, c *gin.Context, agentIdPtr *uuid.UUID, fn func()) {
	if agentIdPtr == nil {
		fn()
		return
	}
	agentId := agentIdPtr.String()

	logger := at.logger.With(zap.String("agent-id", agentId))

	device := &models.Device{}
	_ = api.DeviceIsOwnedByCurrentUser(c, api.db).First(&device, "id = ?", agentId)

	site := &models.Site{}
	_ = api.SiteIsOwnedByCurrentUser(c, api.db).First(&site, "id = ?", agentId)

	if device.ID == uuid.Nil && site.ID == uuid.Nil {
		at.logger.Warn("cannot track: invalid agent id")
		fn()
		return
	}

	if err := at.connected(agentId); err != nil {
		logger.Warn("failed to update online redis state for device", zap.Error(err))
		fn()
		return
	}

	if device.ID != uuid.Nil && !device.Online {
		device.Online = true
		now := time.Now()
		device.OnlineAt = &now
		err := api.db.Select("online", "online_at").Updates(device).Error
		if err != nil {
			logger.Warn("failed to update db state for device", zap.Error(err))
			fn()
		}
	}
	if site.ID != uuid.Nil && !site.Online {
		site.Online = true
		now := time.Now()
		site.OnlineAt = &now
		err := api.db.Select("online", "online_at").Updates(site).Error
		if err != nil {
			logger.Warn("failed to update db state for site", zap.Error(err))
			fn()
		}
	}

	defer func() {
		if err := at.disconnected(agentId); err != nil {
			logger.Warn("failed to update offline redis state for agent", zap.Error(err))
			return
		}
		time.AfterFunc(at.reconnectGracePeriod, func() {
			connected, err := at.isConnected(agentId)
			if err != nil {
				logger.Warn("failed to get online state for agent", zap.Error(err))
				return
			}
			if connected {
				return
			}
			now := time.Now()

			if device.ID != uuid.Nil {
				device.Online = false
				device.OnlineAt = &now
				err = api.db.Select("online", "online_at").Where("online = true").Updates(device).Error
				if err != nil {
					logger.Warn("failed to update db state for device", zap.Error(err))
				}
			}
			if site.ID != uuid.Nil {
				site.Online = false
				site.OnlineAt = &now
				err = api.db.Select("online", "online_at").Where("online = true").Updates(site).Error
				if err != nil {
					logger.Warn("failed to update db state for site", zap.Error(err))
				}
			}
		})
	}()
	fn()
}

func (at *AgentTracker) connected(publicKey string) error {

	at.mu.Lock()
	defer at.mu.Unlock()

	// don't send a subscriptions to redis... it will only track 1 sub.
	if count, found := at.localAgents[publicKey]; found {
		count++
		at.localAgents[publicKey] = count
		return nil
	}

	at.localAgents[publicKey] = 1
	if at.pubSub == nil {
		at.pubSub = at.redis.Subscribe(context.Background(), at.keyPrefix+publicKey)
	}

	err := at.pubSub.Subscribe(context.Background(), at.keyPrefix+publicKey)
	if err != nil {
		return err
	}
	return nil
}

func (at *AgentTracker) disconnected(publicKey string) error {

	at.mu.Lock()
	defer at.mu.Unlock()

	count, found := at.localAgents[publicKey]
	if !found {
		return nil
	}

	count--
	if count != 0 {
		at.localAgents[publicKey] = count
		return nil
	}

	delete(at.localAgents, publicKey)
	err := at.pubSub.Unsubscribe(context.Background(), at.keyPrefix+publicKey)
	if err != nil {
		return err
	}
	return nil
}

func (at *AgentTracker) isConnected(publicKey string) (bool, error) {
	key := at.keyPrefix + publicKey
	result, err := at.redis.PubSubNumSub(context.Background(), key).Result()
	if err != nil {
		return false, err
	}
	return result[key] != 0, nil
}
