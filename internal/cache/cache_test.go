package cache

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// helper creates a DNS response for testing
func makeResponse(domain string, ip string, ttl uint32) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(domain),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		A: net.ParseIP(ip),
	})
	return msg
}

func TestCacheHitAndMiss(t *testing.T) {
	c := New(30*time.Second, 1*time.Hour)

	// Miss on empty cache
	result := c.Get("google.com.", dns.TypeA)
	if result != nil {
		t.Fatal("Expected cache miss, got hit")
	}

	// Store a response
	resp := makeResponse("google.com", "142.250.1.100", 300)
	c.Set("google.com.", dns.TypeA, resp)

	// Should hit now
	result = c.Get("google.com.", dns.TypeA)
	if result == nil {
		t.Fatal("Expected cache hit, got miss")
	}

	if len(result.Answer) == 0 {
		t.Fatal("Cached response has no answers")
	}

	a, ok := result.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("Answer is not type A")
	}

	if a.A.String() != "142.250.1.100" {
		t.Errorf("Expected 142.250.1.100, got %s", a.A.String())
	}
}

func TestCacheTTLFloor(t *testing.T) {
	// Floor = 30s, so even a TTL of 5s should be stored for 30s
	c := New(30*time.Second, 1*time.Hour)

	resp := makeResponse("short-ttl.com", "1.1.1.1", 5) // 5s TTL
	c.Set("short-ttl.com.", dns.TypeA, resp)

	// Should still be cached (floor = 30s)
	result := c.Get("short-ttl.com.", dns.TypeA)
	if result == nil {
		t.Fatal("Expected hit — TTL floor should keep entry alive")
	}
}

func TestCacheExpiration(t *testing.T) {
	// Use very short floor/ceiling for testing
	c := New(50*time.Millisecond, 100*time.Millisecond)

	resp := makeResponse("expire.com", "1.2.3.4", 1)
	c.Set("expire.com.", dns.TypeA, resp)

	// Should hit immediately
	if c.Get("expire.com.", dns.TypeA) == nil {
		t.Fatal("Expected hit immediately after set")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should miss now
	if c.Get("expire.com.", dns.TypeA) != nil {
		t.Fatal("Expected miss after expiration")
	}
}

func TestCacheInvalidate(t *testing.T) {
	c := New(30*time.Second, 1*time.Hour)

	// Cache both A and AAAA for same domain
	respA := makeResponse("example.com", "1.2.3.4", 300)
	c.Set("example.com.", dns.TypeA, respA)
	c.Set("example.com.", dns.TypeAAAA, makeResponse("example.com", "::1", 300))

	if c.Size() != 2 {
		t.Fatalf("Expected 2 entries, got %d", c.Size())
	}

	// Invalidate removes all types for that domain (RF02.4)
	removed := c.Invalidate("example.com")
	if removed != 2 {
		t.Errorf("Expected 2 removed, got %d", removed)
	}

	if c.Get("example.com.", dns.TypeA) != nil {
		t.Fatal("A record should be invalidated")
	}
}

func TestCacheStats(t *testing.T) {
	c := New(30*time.Second, 1*time.Hour)

	c.Get("miss1.com.", dns.TypeA) // miss
	c.Get("miss2.com.", dns.TypeA) // miss

	resp := makeResponse("hit.com", "1.2.3.4", 300)
	c.Set("hit.com.", dns.TypeA, resp)
	c.Get("hit.com.", dns.TypeA) // hit

	stats := c.GetStats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 2 {
		t.Errorf("Expected 2 misses, got %d", stats.Misses)
	}
}

func TestCacheDifferentTypes(t *testing.T) {
	c := New(30*time.Second, 1*time.Hour)

	// A and AAAA for same domain are separate entries
	c.Set("google.com.", dns.TypeA, makeResponse("google.com", "1.2.3.4", 300))
	c.Set("google.com.", dns.TypeAAAA, makeResponse("google.com", "::1", 300))

	if c.Size() != 2 {
		t.Fatalf("Expected 2 entries (A + AAAA), got %d", c.Size())
	}

	if c.Get("google.com.", dns.TypeA) == nil {
		t.Fatal("A record should be cached")
	}
	if c.Get("google.com.", dns.TypeAAAA) == nil {
		t.Fatal("AAAA record should be cached")
	}
}