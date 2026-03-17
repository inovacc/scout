package proxy

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data    []byte
	expires time.Time
}

type responseCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

func newResponseCache() *responseCache {
	return &responseCache{entries: make(map[string]*cacheEntry)}
}

func (c *responseCache) get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}

	return entry.data, true
}

func (c *responseCache) set(key string, data []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &cacheEntry{
		data:    data,
		expires: time.Now().Add(ttl),
	}
}
