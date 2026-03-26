package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

// RedisCache provides L2 caching via Redis (RF02.2).
// Used as fallback when L1 in-memory misses.
type RedisCache struct {
	client   *redis.Client
	ttlFloor time.Duration
	ttlCeil  time.Duration
}

// NewRedisCache creates a Redis L2 cache connection.
// Returns nil if Redis is unavailable (RNF02.2).
func NewRedisCache(addr string, ttlFloor, ttlCeil time.Duration) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolSize:     20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Printf("Redis L2: indisponível (%v) — usando apenas cache L1\n", err)
		return nil
	}

	fmt.Println("Redis L2: conectado em", addr)
	return &RedisCache{
		client:   client,
		ttlFloor: ttlFloor,
		ttlCeil:  ttlCeil,
	}
}

// Get retrieves a DNS response from Redis.
func (r *RedisCache) Get(name string, qtype uint16) *dns.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	k := key(name, qtype)
	data, err := r.client.Get(ctx, k).Bytes()
	if err != nil {
		return nil
	}

	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {
		return nil
	}

	return msg
}

// Set stores a DNS response in Redis with TTL.
func (r *RedisCache) Set(name string, qtype uint16, msg *dns.Msg, ttl time.Duration) {
	if msg == nil || len(msg.Answer) == 0 {
		return
	}

	// Clamp TTL
	if ttl < r.ttlFloor {
		ttl = r.ttlFloor
	}
	if ttl > r.ttlCeil {
		ttl = r.ttlCeil
	}

	data, err := msg.Pack()
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	k := key(name, qtype)
	r.client.Set(ctx, k, data, ttl)
}

// Close shuts down the Redis connection.
func (r *RedisCache) Close() {
	if r.client != nil {
		r.client.Close()
	}
}