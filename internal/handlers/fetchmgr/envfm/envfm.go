package envfm

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/memfm"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/nonefm"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/redisfm"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"os"
	"strconv"
	"time"
)

// New chooses a fetchmgr.FetchManager implementation based on environment variable configuration
func New(logger *zap.Logger) (fetchmgr.FetchManager, error) {

	fetchMgr := Getenv("NEXAPI_FETCH_MGR", "redis")
	switch fetchMgr {
	case "none":
		return nonefm.New(), nil
	case "memfm":
		return memfm.New(), nil
	case "redis":

		timeout, err := GetenvDuration("NEXAPI_FETCH_MGR_TIMEOUT", "2s")
		if err != nil {
			return nil, err
		}

		redisAddr := Getenv("NEXAPI_REDIS_SERVER", "redis:6379")
		redisDB, err := GetenvInt("NEXAPI_REDIS_DB", "1")
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
func Getenv(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetenvDuration(name, defaultValue string) (time.Duration, error) {
	valueStr := Getenv(name, defaultValue)
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return value, fmt.Errorf("invalid value for %s (%s): %w", name, valueStr, err)
	}
	return value, nil
}

func GetenvInt(name, defaultValue string) (int, error) {
	valueStr := Getenv(name, defaultValue)
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return value, fmt.Errorf("invalid value for %s (%s): %w", name, valueStr, err)
	}
	return value, nil
}
