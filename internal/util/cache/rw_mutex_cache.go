package cache

import (
	"sync"
)

type RWMutexCache[K comparable, V any] struct {
	data map[K]V
	mu   sync.RWMutex
}

func NewRWMutexCache[K comparable, V any]() *RWMutexCache[K, V] {
	return &RWMutexCache[K, V]{
		data: map[K]V{},
	}
}

func (c *RWMutexCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	x, found := c.data[key]
	return x, found
}
func (c *RWMutexCache[K, V]) Put(key K, value V) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, found := c.data[key]
	c.data[key] = value
	return x, found
}

func (c *RWMutexCache[K, V]) Delete(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, found := c.data[key]
	delete(c.data, key)
	return x, found
}
