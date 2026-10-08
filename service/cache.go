package service

import (
	"sync"
	"time"
)

// Cache is a tiny in-memory TTL store for expensive read endpoints
// (report overview). Entries expire lazily on read; a cleanup pass
// runs at most once per TTL to bound memory.
type Cache struct {
	mu        sync.Mutex
	ttl       time.Duration
	items     map[string]cacheItem
	lastSweep time.Time
	now       func() time.Time
}

type cacheItem struct {
	value   any
	expires time.Time
}

// NewCache builds a store whose entries live for ttl.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, items: map[string]cacheItem{}, now: time.Now}
}

// Get returns the value when present and fresh.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	it, ok := c.items[key]
	if !ok || !c.now().Before(it.expires) {
		delete(c.items, key)
		return nil, false
	}
	return it.value, true
}

// Set stores value for one ttl. Expensive values should be immutable
// (or treated as read-only) since Get shares the reference.
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cacheItem{value: value, expires: c.now().Add(c.ttl)}
	if c.now().Sub(c.lastSweep) > c.ttl {
		for k, it := range c.items {
			if !c.now().Before(it.expires) {
				delete(c.items, k)
			}
		}
		c.lastSweep = c.now()
	}
}

// Invalidate drops one entry early.
func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}
