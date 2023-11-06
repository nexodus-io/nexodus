package routers

import (
	"context"
	"sync/atomic"
)

type UserLimiters struct {
	counter int32
	Single  Limiter
}

func NewUserLimiters() *UserLimiters {
	return &UserLimiters{
		Single: NewLimiter(1),
	}
}

func (c *UserLimiters) Add() {
	if c == nil {
		return
	}
	atomic.AddInt32(&c.counter, 1)
}
func (c *UserLimiters) Done() int32 {
	return atomic.AddInt32(&c.counter, -1)
}

type Limiter struct {
	limit chan struct{}
}

func NewLimiter(maxConcurrency int) Limiter {
	serializer := make(chan struct{}, maxConcurrency)
	return Limiter{
		limit: serializer,
	}
}

func (c *Limiter) Do(ctx context.Context, f func()) (canceled bool) {

	select {
	case c.limit <- struct{}{}:
		defer func() {
			<-c.limit
		}()
		f()
		canceled = false
	case <-ctx.Done():
		canceled = true
	}
	return
}
