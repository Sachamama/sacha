package cache

import (
	"sync"
)

// Key identifies a cache entry by account, region, and service.
type Key struct {
	AccountID string
	Region    string
	Service   string
}

// Cache is a thread-safe in-memory cache for service resource lists.
// It stores items keyed by {AccountID, Region, Service} so that switching
// between services/regions can show previously loaded data instantly.
type Cache struct {
	mu      sync.RWMutex
	entries map[Key]interface{}
}

// New creates an empty cache.
func New() *Cache {
	return &Cache{
		entries: make(map[Key]interface{}),
	}
}

// Get retrieves cached items for the given key.
// Returns the items and true if found, nil and false otherwise.
func (c *Cache) Get(key Key) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.entries[key]
	return v, ok
}

// Set stores items in the cache under the given key.
func (c *Cache) Set(key Key, items interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = items
}

// Delete removes a single cache entry.
func (c *Cache) Delete(key Key) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[Key]interface{})
}

// Len returns the number of entries in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
