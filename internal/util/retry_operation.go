package util

import (
	"context"
	"errors"
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

// RetryOperationForErrors retries the operation with a backoff policy for the specified errors, otherwise will just perform the operation once and return the error if it fails.
func RetryOperationForErrors(ctx context.Context, wait time.Duration, retries int, retriableErrors []error, operation func() error) error {
	bo := backoff.WithMaxRetries(
		backoff.NewConstantBackOff(wait),
		uint64(retries),
	)
	bo = backoff.WithContext(bo, ctx)

	err := backoff.Retry(func() error {
		err := operation()
		for _, retriableError := range retriableErrors {
			if errors.Is(err, retriableError) {
				return err
			}
		}
		if err != nil {
			return backoff.Permanent(err)
		}
		return nil
	}, bo)

	return err
}
