package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Retention manages automatic cleanup of old log partitions (RF07.5).
type Retention struct {
	pool     *pgxpool.Pool
	maxAge   time.Duration
	interval time.Duration
}

// NewRetention creates a retention manager.
// maxAge defines how long logs are kept (default: 120 days).
// interval defines how often cleanup runs (default: 24h).
func NewRetention(pool *pgxpool.Pool, maxAge time.Duration) *Retention {
	return &Retention{
		pool:     pool,
		maxAge:   maxAge,
		interval: 24 * time.Hour,
	}
}

// Start launches the background goroutine that drops old chunks.
func (r *Retention) Start() {
	// Run once at startup
	r.cleanup()

	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for range ticker.C {
			r.cleanup()
		}
	}()

	fmt.Printf("Log retention: started (max_age=%v, check_interval=%v)\n",
		r.maxAge, r.interval)
}

// cleanup drops TimescaleDB chunks older than maxAge (RF07.5, RF07.6).
func (r *Retention) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var dropped int
	err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM drop_chunks('dns_query_logs', older_than => $1::interval)",
		fmt.Sprintf("%d days", int(r.maxAge.Hours()/24)),
	).Scan(&dropped)

	if err != nil {
		fmt.Printf("Log retention: cleanup failed: %v\n", err)
		return
	}

	if dropped > 0 {
		fmt.Printf("Log retention: dropped %d old chunks (older than %v)\n",
			dropped, r.maxAge)
	}
}
