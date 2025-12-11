package cache

import (
	"sync"
	"time"
)

type Entry[T any] struct {
	Value      T
	Expiration time.Time
}

type Cache[K any, V any] struct {
	m sync.Map
}

func (c *Cache[K, V]) Store(key K, value V, timeout time.Duration) {
	c.m.Store(key, &Entry[V]{Value: value, Expiration: time.Now().Add(timeout)})
}

func (c *Cache[K, V]) Load(key K) (value V, found bool) {
	now := time.Now()

	rawEntry, found := c.m.Load(key)
	if !found {
		return value, false
	}

	entry := rawEntry.(*Entry[V])
	if now.After(entry.Expiration) {
		return value, false
	}
	return entry.Value, true
}
