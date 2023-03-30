package cache

import "time"

type memoizeResult[V any] struct {
	value V
	err   error
}

type MemoizeCache[K comparable, V any] struct {
	data     *RWMutexTTLCache[K, memoizeResult[V]]
	errorTTL time.Duration
}

func NewMemoizeCache[K comparable, V any](defaultTTL time.Duration, errorTTL time.Duration) *MemoizeCache[K, V] {
	return &MemoizeCache[K, V]{
		data:     NewRWMutexTTLCache[K, memoizeResult[V]](defaultTTL),
		errorTTL: errorTTL,
	}
}

func (c *MemoizeCache[K, V]) Memoize(key K, f func() V) V {
	return c.MemoizeWithTTL(key, c.data.DefaultTTL, f)
}

func (c *MemoizeCache[K, V]) MemoizeCanErr(key K, f func() (V, error)) (V, error) {
	return c.MemoizeCanErrWithTTL(key, c.data.DefaultTTL, c.errorTTL, f)
}

func (c *MemoizeCache[K, V]) MemoizeWithTTL(key K, ttl time.Duration, f func() V) V {
	value, _ := c.MemoizeCanErrWithTTL(key, ttl, ttl, func() (V, error) {
		return f(), nil
	})
	return value
}

func (c *MemoizeCache[K, V]) MemoizeCanErrWithTTL(key K, ttl time.Duration, errorTTL time.Duration, f func() (V, error)) (V, error) {
	if result, found := c.data.Get(key); found {
		return result.value, result.err
	}
	res, err := f()
	if err != nil {
		ttl = errorTTL
	}
	_, _ = c.data.PutWithTTL(key, memoizeResult[V]{
		value: res,
		err:   err,
	}, ttl)
	return res, err
}
