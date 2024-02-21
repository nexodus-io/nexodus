//go:build integration

package integration_tests

import (
	"context"
	"errors"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/redisfm"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/tests"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestFetchManager(t *testing.T) {

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "-n", "nexodus", "port-forward", "redis-0", "6379:6379")
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	defer func() {
		cancel() // to kill the process
		_ = cmd.Wait()
	}()
	require.NoError(t, err)

	logger, _ := zap.NewProductionConfig().Build()

	client := redis.NewClient(&redis.Options{
		Addr:             "127.0.0.1:6379",
		DisableIndentity: true,
	})
	for i := 0; i < 10; i++ {
		_, err = client.Del(context.Background(), "key1").Result()
		if err != nil {
			if strings.Contains(err.Error(), "connect: connection refused") {
				// the port forward is not up and running yet... try agiant in a bit.
				fmt.Println(err, ", will retry in 100ms")
				time.Sleep(100 * time.Millisecond)
				continue
			} else if !errors.Is(err, redis.Nil) {
				// this one happens when key1 does not exist.
				require.NoError(t, err)
			}
		}
	}
	_ = client.Close()

	clients := []*redis.Client{}
	managers := []fetchmgr.FetchManager{}
	for i := 0; i < 5; i++ {
		client := redis.NewClient(&redis.Options{
			Addr:             "127.0.0.1:6379",
			DisableIndentity: true,
		})
		clients = append(clients, client)
		manager := redisfm.New(client, 1000*time.Second, logger, "")
		managers = append(managers, manager)
	}
	defer func() {
		for _, client := range clients {
			_ = client.Close()
		}
	}()

	tests.TestFetchManagerReducesDBFetchesAtTheTail(t, managers...)
}
