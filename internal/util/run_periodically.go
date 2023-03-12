package util

import (
	"context"
	"time"
)

func RunPeriodically(ctx context.Context, duration time.Duration, fn func()) {
	_, _ = CheckPeriodically(ctx, duration, func() (bool, error) {
		fn()
		return false, nil
	})
}

// CheckPeriodically checks a function (fn) at a specified interval (duration).
// It will return when one of these conditions occurs: fn returns true, fn returns
// an error, the duration is met, or the context is Done().
func CheckPeriodically(ctx context.Context, duration time.Duration, fn func() (bool, error)) (bool, error) {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, nil
		case <-ticker.C:
			cond, err := fn()
			if cond || err != nil {
				return cond, err
			}
		}
	}
}
