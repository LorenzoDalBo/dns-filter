package cache

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

// entry holds a cached DNS response with its expiration time.
type entry struct {
	msg       *dns.Msg
	expiresAt time.Time
}

// Stats tracks cache hit/miss counters for the dashboard (RF02.5).
type Stats struct {
	Hits   uint64
	Misses uint64
}

// Cache provides in-memory DNS response caching with TTL (RF02.1).
// This is the L1 cache. L2 Redis will be added in a future phase.
type Cache struct {
	mu       sync.RWMutex
	entries  map[string]*entry
	ttlFloor time.Duration
	ttlCeil  time.Duration
	hits     atomic.Uint64
	misses   atomic.Uint64
	l2       *RedisCache
}

func New(ttlFloor, ttlCeil time.Duration) *Cache {
	c := &Cache{
		entries:  make(map[string]*entry),
		ttlFloor: ttlFloor,
		ttlCeil:  ttlCeil,
	}

	// Background goroutine to evict expired entries every 30s.
	// Prevents unbounded memory growth from stale entries.
	go c.evictLoop()

	return c
}

// SetL2 attaches a Redis L2 cache (RF02.2).
func (c *Cache) SetL2(l2 *RedisCache) {
	c.l2 = l2
}

// key builds the cache key from domain + query type.
// Example: "google.com.|A"
func key(name string, qtype uint16) string {
	return strings.ToLower(name) + "|" + dns.TypeToString[qtype]
}

// Get looks up a cached response. Returns nil on miss or expiry.
func (c *Cache) Get(name string, qtype uint16) *dns.Msg {
	k := key(name, qtype)

	// L1 lookup
	c.mu.RLock()
	e, found := c.entries[k]
	c.mu.RUnlock()

	if found && time.Now().Before(e.expiresAt) {
		c.hits.Add(1)
		return e.msg.Copy()
	}

	// L2 lookup (Redis)
	if c.l2 != nil {
		if msg := c.l2.Get(name, qtype); msg != nil {
			// Promote to L1
			c.Set(name, qtype, msg)
			c.hits.Add(1)
			return msg.Copy()
		}
	}

	c.misses.Add(1)
	return nil
}

// Set stores a DNS response in the cache.
// TTL is extracted from the response, clamped to [floor, ceiling].
func (c *Cache) Set(name string, qtype uint16, msg *dns.Msg) {
	if msg == nil || len(msg.Answer) == 0 {
		return
	}

	ttl := c.extractTTL(msg)

	k := key(name, qtype)

	c.mu.Lock()
	c.entries[k] = &entry{
		msg:       msg.Copy(),
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()

	// Write-through to L2 (Redis)
	if c.l2 != nil {
		c.l2.Set(name, qtype, msg, ttl)
	}
}

// Invalidate removes a specific domain from cache (RF02.4).
// Removes all query types for that domain.
func (c *Cache) Invalidate(name string) int {
	name = strings.ToLower(dns.Fqdn(name))
	removed := 0

	c.mu.Lock()
	for k := range c.entries {
		if strings.HasPrefix(k, name+"|") {
			delete(c.entries, k)
			removed++
		}
	}
	c.mu.Unlock()

	if removed > 0 {
		fmt.Printf("Cache: invalidated %d entries for %s\n", removed, name)
	}
	return removed
}

// GetStats returns current hit/miss counters (RF02.5).
func (c *Cache) GetStats() Stats {
	return Stats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
	}
}

// Size returns the number of entries currently in cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Clear removes all entries from the L1 cache.
// Called after blacklist reload to prevent serving stale allowed responses.
func (c *Cache) Clear() {
	c.mu.Lock()
	count := len(c.entries)
	c.entries = make(map[string]*entry)
	c.mu.Unlock()

	if c.l2 != nil {
		c.l2.Clear()
	}

	if count > 0 {
		fmt.Printf("Cache: cleared %d entries\n", count)
	}
}

// extractTTL finds the minimum TTL across all answer records,
// then clamps it between floor and ceiling (RF02.3).
func (c *Cache) extractTTL(msg *dns.Msg) time.Duration {
	var minTTL uint32 = 3600 // default 1h if no records

	for _, rr := range msg.Answer {
		if ttl := rr.Header().Ttl; ttl < minTTL {
			minTTL = ttl
		}
	}

	ttl := time.Duration(minTTL) * time.Second

	// Clamp to [floor, ceiling]
	if ttl < c.ttlFloor {
		ttl = c.ttlFloor
	}
	if ttl > c.ttlCeil {
		ttl = c.ttlCeil
	}

	return ttl
}

// evictLoop runs every 30s and removes expired entries.
func (c *Cache) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		removed := 0

		c.mu.Lock()
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
				removed++
			}
		}
		c.mu.Unlock()

		if removed > 0 {
			fmt.Printf("Cache: evicted %d expired entries (%d remaining)\n", removed, c.Size())
		}
	}
}
