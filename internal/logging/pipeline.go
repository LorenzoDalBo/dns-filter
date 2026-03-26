package logging

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pipeline implements the async log writer (RF07.2-RF07.4).
// DNS handler sends entries via fire-and-forget channel.
// Pipeline accumulates and batch-inserts into TimescaleDB.
type Pipeline struct {
	ch            chan Entry
	pool          *pgxpool.Pool
	wg            sync.WaitGroup
	batchSize     int
	flushInterval time.Duration
	dropped       uint64
}

// NewPipeline creates a log pipeline with a buffered channel.
// bufferSize controls how many entries can queue before dropping (RF07.4).
func NewPipeline(pool *pgxpool.Pool, bufferSize int) *Pipeline {
	return &Pipeline{
		ch:            make(chan Entry, bufferSize),
		pool:          pool,
		batchSize:     5000,            // RF07.3: flush at 5000 entries
		flushInterval: 1 * time.Second, // RF07.3: or every 1 second
	}
}

// Send queues a log entry without blocking (RF07.2).
// If the buffer is full, the entry is silently dropped (RF07.4).
func (p *Pipeline) Send(entry Entry) {
	select {
	case p.ch <- entry:
		// queued successfully
	default:
		// buffer full — drop silently, never block DNS (RF07.4)
		p.dropped++
		if p.dropped%1000 == 1 {
			fmt.Printf("Log pipeline: buffer full, dropped %d entries\n", p.dropped)
		}
	}
}

// Start launches the background consumer goroutine.
// Call this once at server startup.
func (p *Pipeline) Start() {
	p.wg.Add(1)
	go p.consume()
	fmt.Printf("Log pipeline: started (buffer=%d, batch=%d, flush=%v)\n",
		cap(p.ch), p.batchSize, p.flushInterval)
}

// Stop signals the pipeline to flush remaining entries and exit.
func (p *Pipeline) Stop() {
	close(p.ch)
	p.wg.Wait()
	fmt.Printf("Log pipeline: stopped (total dropped: %d)\n", p.dropped)
}

// Pending returns the number of entries waiting to be written.
func (p *Pipeline) Pending() int {
	return len(p.ch)
}

// consume is the background goroutine that reads from the channel
// and batch-inserts into PostgreSQL.
func (p *Pipeline) consume() {
	defer p.wg.Done()

	batch := make([]Entry, 0, p.batchSize)
	ticker := time.NewTicker(p.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-p.ch:
			if !ok {
				// Channel closed — flush remaining and exit
				if len(batch) > 0 {
					p.flush(batch)
				}
				return
			}

			batch = append(batch, entry)

			// Flush when batch is full (RF07.3)
			if len(batch) >= p.batchSize {
				p.flush(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// Flush by time interval (RF07.3)
			if len(batch) > 0 {
				p.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

// flush writes a batch of entries to PostgreSQL.
// flush writes a batch of entries to PostgreSQL using a single multi-row INSERT.
func (p *Pipeline) flush(batch []Entry) {
	if p.pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build batch insert for much better performance than individual INSERTs
	b := &pgx.Batch{}
	for _, e := range batch {
		b.Queue(`
			INSERT INTO dns_query_logs
				(queried_at, client_ip, user_id, group_id, domain, query_type,
				 action, block_reason, category_id, response_ip, response_ms, upstream)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`,
			e.QueriedAt, e.ClientIP.String(), e.UserID, e.GroupID,
			e.Domain, e.QueryType, e.Action, e.BlockReason,
			e.CategoryID, ipToString(e.ResponseIP), e.ResponseMs, e.Upstream,
		)
	}

	results := p.pool.SendBatch(ctx, b)
	defer results.Close()

	var failed int
	for range batch {
		if _, err := results.Exec(); err != nil {
			failed++
		}
	}

	if failed > 0 {
		fmt.Printf("Log pipeline: flushed %d entries (%d failed)\n", len(batch)-failed, failed)
	} else {
		fmt.Printf("Log pipeline: flushed %d entries\n", len(batch))
	}
}

func ipToString(ip net.IP) *string {
	if ip == nil {
		return nil
	}
	s := ip.String()
	return &s
}
