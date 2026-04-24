package ai

import (
	"sync"
	"time"
)

// CacheKey uniquely identifies a PR summary.
// Using URL + UpdatedAt + ViewerReviewState means the cache auto-invalidates when the PR changes
// or when the viewer's review state transitions (e.g. after approving).
type CacheKey struct {
	URL               string
	UpdatedAt         time.Time
	ViewerReviewState string
}

// Cache is a thread-safe in-memory LRU-eviction cache.
type Cache[V any] struct {
	mu      sync.RWMutex
	entries map[CacheKey]V
	order   []CacheKey // insertion order for simple eviction
	maxSize int
}

// NewCache creates a new Cache with the given maximum size.
func NewCache[V any](maxSize int) *Cache[V] {
	return &Cache[V]{
		entries: make(map[CacheKey]V),
		maxSize: maxSize,
	}
}

func (c *Cache[V]) Get(key CacheKey) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.entries[key]
	return v, ok
}

func (c *Cache[V]) Set(key CacheKey, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists {
		if len(c.entries) >= c.maxSize {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.entries, oldest)
		}
		c.order = append(c.order, key)
	}
	c.entries[key] = value
}

// SummaryCache is a type alias for Cache[PRSummaryResponse] for backward compatibility.
type SummaryCache = Cache[PRSummaryResponse]

// NewSummaryCache creates a SummaryCache with the given maximum size.
func NewSummaryCache(maxSize int) *SummaryCache {
	return NewCache[PRSummaryResponse](maxSize)
}
