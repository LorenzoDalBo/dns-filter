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
	ttlFloor time.Duration // minimum TTL applied to any entry (RF02.3)
	ttlCeil  time.Duration // maximum TTL applied to any entry (RF02.3)
	hits     atomic.Uint64
	misses   atomic.Uint64
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

// key builds the cache key from domain + query type.
// Example: "google.com.|A"
func key(name string, qtype uint16) string {
	return strings.ToLower(name) + "|" + dns.TypeToString[qtype]
}

// Get looks up a cached response. Returns nil on miss or expiry.
func (c *Cache) Get(name string, qtype uint16) *dns.Msg {
	k := key(name, qtype)

	c.mu.RLock()
	e, found := c.entries[k]
	c.mu.RUnlock()

	if !found || time.Now().After(e.expiresAt) {
		c.misses.Add(1)
		return nil
	}

	c.hits.Add(1)

	// Return a copy so callers don't mutate the cached message.
	// dns.Msg.Copy() is provided by miekg/dns for exactly this purpose.
	return e.msg.Copy()
}

// Set stores a DNS response in the cache.
// TTL is extracted from the response, clamped to [floor, ceiling].
func (c *Cache) Set(name string, qtype uint16, msg *dns.Msg) {
	if msg == nil || len(msg.Answer) == 0 {
		return // don't cache empty responses
	}

	ttl := c.extractTTL(msg)

	k := key(name, qtype)

	c.mu.Lock()
	c.entries[k] = &entry{
		msg:       msg.Copy(),
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
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
