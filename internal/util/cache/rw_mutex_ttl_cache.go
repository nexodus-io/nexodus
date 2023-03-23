package cache

import (
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}
type RWMutexTTLCache[K comparable, V any] struct {
	data       *RWMutexCache[K, entry[V]]
	DefaultTTL time.Duration
}

func NewRWMutexTTLCache[K comparable, V any](defaultTTL time.Duration) *RWMutexTTLCache[K, V] {
	return &RWMutexTTLCache[K, V]{
		data:       NewRWMutexCache[K, entry[V]](),
		DefaultTTL: defaultTTL,
	}
}

func (c *RWMutexTTLCache[K, V]) Get(key K) (V, bool) {
	x, found := c.data.Get(key)
	if found && x.expiresAt.Before(time.Now()) {
		x = entry[V]{}
		found = false
	}
	return x.value, found
}

func (c *RWMutexTTLCache[K, V]) PutWithTTL(key K, value V, ttl time.Duration) (V, bool) {
	x, found := c.data.Put(key, entry[V]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	})
	if found && x.expiresAt.Before(time.Now()) {
		x = entry[V]{}
		found = false
	}
	return x.value, found
}

func (c *RWMutexTTLCache[K, V]) Put(key K, value V) (V, bool) {
	return c.PutWithTTL(key, value, c.DefaultTTL)
}

func (c *RWMutexTTLCache[K, V]) Delete(key K) (V, bool) {
	x, found := c.data.Delete(key)
	if found && x.expiresAt.Before(time.Now()) {
		x = entry[V]{}
		found = false
	}
	return x.value, found
}
