package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides database access to all tables.
// Uses a connection pool for concurrent access from DNS handler goroutines.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store with a connection pool to PostgreSQL.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: parse config: %w", err)
	}

	// Pool settings tuned for DNS server workload
	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close shuts down the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// Pool exposes the underlying pool for direct queries when needed.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}
