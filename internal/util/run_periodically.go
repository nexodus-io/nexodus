package util

import (
	"context"
	"time"
)

func RunPeriodically(ctx context.Context, duration time.Duration, fn func()) {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}
