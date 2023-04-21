package util

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// RetryOperation retries the operation with a backoff policy.
func RetryOperation(ctx context.Context, wait time.Duration, retries int, operation func() error) error {
	bo := backoff.WithMaxRetries(
		backoff.NewConstantBackOff(wait),
		uint64(retries),
	)
	bo = backoff.WithContext(bo, ctx)
	err := backoff.Retry(operation, bo)

	return err
}
