package web

import (
	"sync"
	"time"
)

// cache is a small in-memory map with per-entry expiry. When full, expired
// entries are dropped first, then the entry closest to expiry.
type cache[K comparable, V any] struct {
	mu      sync.Mutex
	max     int
	entries map[K]cacheEntry[V]
	now     func() time.Time
}

type cacheEntry[V any] struct {
	value   V
	expires time.Time
}

func newCache[K comparable, V any](max int) *cache[K, V] {
	return &cache[K, V]{max: max, entries: map[K]cacheEntry[V]{}, now: time.Now}
}

func (c *cache[K, V]) get(k K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[k]
	if !ok || c.now().After(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *cache[K, V]) put(k K, v V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if _, ok := c.entries[k]; !ok && len(c.entries) >= c.max {
		var oldest K
		var oldestExp time.Time
		first := true
		for key, e := range c.entries {
			if now.After(e.expires) {
				delete(c.entries, key)
				continue
			}
			if first || e.expires.Before(oldestExp) {
				oldest, oldestExp, first = key, e.expires, false
			}
		}
		if len(c.entries) >= c.max {
			delete(c.entries, oldest)
		}
	}
	c.entries[k] = cacheEntry[V]{value: v, expires: now.Add(ttl)}
}
