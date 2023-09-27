package envfm

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/memfm"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/nonefm"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/redisfm"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// New chooses a fetchmgr.FetchManager implementation based on environment variable configuration
func New(logger *zap.Logger) (fetchmgr.FetchManager, error) {

	fetchMgr := util.Getenv("NEXAPI_FETCH_MGR", "redis")
	switch fetchMgr {
	case "none":
		return nonefm.New(), nil
	case "memfm":
		return memfm.New(), nil
	case "redis":

		timeout, err := util.GetenvDuration("NEXAPI_FETCH_MGR_TIMEOUT", "2s")
		if err != nil {
			return nil, err
		}

		redisAddr := util.Getenv("NEXAPI_REDIS_SERVER", "redis:6379")
		redisDB, err := util.GetenvInt("NEXAPI_REDIS_DB", "1")
		if err != nil {
			return nil, err
		}

		redisClient := redis.NewClient(&redis.Options{
			Addr: redisAddr,
			DB:   redisDB,
		})

		return redisfm.New(redisClient, timeout, logger, "fetchmgr:"), nil
	default:
		return nil, fmt.Errorf("invalid value for NEXAPI_FETCH_MGR (%s)", fetchMgr)
	}
}
