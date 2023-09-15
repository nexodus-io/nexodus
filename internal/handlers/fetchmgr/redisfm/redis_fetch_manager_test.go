package redisfm

import (
	"context"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/tests"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io"
	"os/exec"
	"testing"
	"time"
)

func TestFetchManager(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "-n", "nexodus", "port-forward", "redis-0", "6379:6379")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	err := cmd.Start()
	require.NoError(err)

	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
	})

	// reset any previous data...
	_, _ = client.Del(context.Background(), "key1").Result()

	logger, _ := zap.NewProductionConfig().Build()
	tests.TestFetchManagerReducesDBFetchesAtTheTail(t, New(client, 1000*time.Second, logger, ""))
}
