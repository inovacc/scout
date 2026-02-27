package scout

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// ResearchDepth provides named presets for research thoroughness.
type ResearchDepth int

const (
	// ResearchShallow performs a single search+fetch pass (depth 1, 3 sources).
	ResearchShallow ResearchDepth = iota
	// ResearchMedium performs one follow-up pass (depth 2, 5 sources).
	ResearchMedium
	// ResearchDeep performs multiple follow-up passes (depth 3, 8 sources).
	ResearchDeep
)

// WithResearchPreset applies a named depth preset.
func WithResearchPreset(depth ResearchDepth) ResearchOption {
	return func(o *researchOpts) {
		switch depth {
		case ResearchShallow:
			o.maxDepth = 1
			o.maxSources = 3
			o.concurrency = 2
		case ResearchMedium:
			o.maxDepth = 2
			o.maxSources = 5
			o.concurrency = 3
		case ResearchDeep:
			o.maxDepth = 3
			o.maxSources = 8
			o.concurrency = 5
			o.timeout = 5 * time.Minute
		}
	}
}

// ResearchCache caches research results with TTL.
type ResearchCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	result  *ResearchResult
	created time.Time
}

// NewResearchCache creates a cache with the given TTL.
func NewResearchCache(ttl time.Duration) *ResearchCache {
	return &ResearchCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// Get returns a cached result if it exists and hasn't expired.
func (c *ResearchCache) Get(query string) (*ResearchResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := cacheKey(query)
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.created) > c.ttl {
		return nil, false
	}
	return entry.result, true
}

// Put stores a result in the cache.
func (c *ResearchCache) Put(query string, result *ResearchResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[cacheKey(query)] = &cacheEntry{
		result:  result,
		created: time.Now(),
	}
}

// Clear removes all cached entries.
func (c *ResearchCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

// Evict removes expired entries.
func (c *ResearchCache) Evict() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for k, v := range c.entries {
		if time.Since(v.created) > c.ttl {
			delete(c.entries, k)
			count++
		}
	}
	return count
}

// Size returns the number of cached entries.
func (c *ResearchCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func cacheKey(query string) string {
	h := sha256.Sum256([]byte(query))
	return hex.EncodeToString(h[:])
}

// WithResearchCache attaches a cache to the research agent.
// Cached results are returned without performing new searches.
func WithResearchCache(cache *ResearchCache) ResearchOption {
	return func(o *researchOpts) { o.cache = cache }
}

// WithResearchPrior provides a previous result to build upon incrementally.
// The agent will use prior sources to avoid re-fetching and focus on new information.
func WithResearchPrior(prior *ResearchResult) ResearchOption {
	return func(o *researchOpts) { o.prior = prior }
}
