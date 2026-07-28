package api

import (
	"sync"
	"time"
)

// cacheEntry holds a cached Email and its last access time for LRU eviction.
type cacheEntry struct {
	email      *Email
	lastAccess time.Time
}

// lruCache is a bounded, mutex-protected in-memory cache for Email objects.
// When the number of entries exceeds maxSize, the least-recently-used entry is evicted.
type lruCache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
	maxSize int
}

// newLRUCache creates a new lruCache with the given maximum size.
func newLRUCache(maxSize int) *lruCache {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &lruCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
	}
}

// Load retrieves a copy of an Email from the cache by ID. Returning a copy
// keeps callers from mutating cache internals outside the lock.
// Updates the entry's lastAccess time. Returns (nil, false) if not found.
func (c *lruCache) Load(id string) (*Email, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[id]
	if !ok {
		return nil, false
	}
	entry.lastAccess = time.Now()
	cp := *entry.email
	return &cp, true
}

// UpdateUnread sets the IsUnread flag on a cached entry, if present.
func (c *lruCache) UpdateUnread(id string, unread bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[id]; ok {
		entry.email.IsUnread = unread
		entry.lastAccess = time.Now()
	}
}

// Store inserts or updates an Email in the cache.
// IMPORTANT: Never overwrites a FullLoaded=true entry with a FullLoaded=false entry.
// Instead, only refreshes mutable fields (IsUnread, Labels) on the existing entry.
func (c *lruCache) Store(email *Email) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.entries[email.ID]; ok {
		if existing.email.FullLoaded && !email.FullLoaded {
			// Refresh only mutable fields; preserve the full body/attachments.
			existing.email.IsUnread = email.IsUnread
			existing.email.Labels = email.Labels
			existing.lastAccess = time.Now()
			return
		}
	}
	cp := *email
	c.entries[email.ID] = &cacheEntry{
		email:      &cp,
		lastAccess: time.Now(),
	}
	if len(c.entries) > c.maxSize {
		c.evictLRU()
	}
}

// Delete removes an entry from the cache by ID.
func (c *lruCache) Delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, id)
}

// Size returns the current number of cached entries.
func (c *lruCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// evictLRU removes the least-recently-used entry from the cache.
// Must be called with c.mu held.
func (c *lruCache) evictLRU() {
	var oldestID string
	var oldestTime time.Time
	first := true
	for id, entry := range c.entries {
		if first || entry.lastAccess.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.lastAccess
			first = false
		}
	}
	if oldestID != "" {
		delete(c.entries, oldestID)
	}
}
