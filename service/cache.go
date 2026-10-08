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
	inflight  map[string]*call
	lastSweep time.Time
	now       func() time.Time
}

// call tracks one in-flight computation so concurrent misses for the
// same key share a single execution (single-flight).
type call struct {
	wg  sync.WaitGroup
	val any
	err error
}

type cacheItem struct {
	value   any
	expires time.Time
}

// NewCache builds a store whose entries live for ttl.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, items: map[string]cacheItem{}, inflight: map[string]*call{}, now: time.Now}
}

// Do returns the cached value when fresh; otherwise it runs fn exactly
// once per key even under concurrent misses, caches successes and
// shares the result with every waiter. Errors are never cached.
func (c *Cache) Do(key string, fn func() (any, error)) (any, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	c.mu.Lock()
	if cl, dup := c.inflight[key]; dup {
		c.mu.Unlock()
		cl.wg.Wait()
		return cl.val, cl.err
	}
	cl := &call{}
	cl.wg.Add(1)
	c.inflight[key] = cl
	c.mu.Unlock()
	cl.val, cl.err = fn()
	if cl.err == nil {
		c.Set(key, cl.val)
	}
	c.mu.Lock()
	delete(c.inflight, key)
	c.mu.Unlock()
	cl.wg.Done()
	return cl.val, cl.err
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
