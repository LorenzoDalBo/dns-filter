package logging

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://dnsfilter:dnsfilter123@localhost:5432/dnsfilter?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	return pool
}

func TestPipelineFlush(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	// Clean up any previous test data
	pool.Exec(context.Background(), "DELETE FROM dns_query_logs WHERE domain = 'pipeline-test.com.'")

	p := NewPipeline(pool, 1000)
	p.Start()

	// Send some test entries
	for i := 0; i < 5; i++ {
		p.Send(Entry{
			QueriedAt:   time.Now(),
			ClientIP:    net.ParseIP("192.168.1.100"),
			GroupID:     1,
			Domain:      "pipeline-test.com.",
			QueryType:   1, // A
			Action:      ActionAllowed,
			BlockReason: BlockReasonNone,
			ResponseIP:  net.ParseIP("142.250.1.100"),
			ResponseMs:  3.5,
			Upstream:    "8.8.8.8:53",
		})
	}

	// Wait for flush (ticker fires every 1s)
	time.Sleep(2 * time.Second)

	// Verify entries were written
	var count int
	err := pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM dns_query_logs WHERE domain = 'pipeline-test.com.'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 log entries, got %d", count)
	}
	t.Logf("Verified %d entries in database", count)

	p.Stop()

	// Clean up
	pool.Exec(context.Background(), "DELETE FROM dns_query_logs WHERE domain = 'pipeline-test.com.'")
}

func TestPipelineDropOnFullBuffer(t *testing.T) {
	// Small buffer to test dropping
	p := NewPipeline(nil, 3) // nil pool — entries are dropped on flush

	// Fill the buffer
	for i := 0; i < 3; i++ {
		p.Send(Entry{
			QueriedAt: time.Now(),
			ClientIP:  net.ParseIP("10.0.0.1"),
			GroupID:   1,
			Domain:    "test.com.",
			QueryType: 1,
			Action:    ActionAllowed,
		})
	}

	// This one should be dropped (buffer full)
	p.Send(Entry{
		QueriedAt: time.Now(),
		ClientIP:  net.ParseIP("10.0.0.1"),
		GroupID:   1,
		Domain:    "dropped.com.",
		QueryType: 1,
		Action:    ActionAllowed,
	})

	if p.Pending() != 3 {
		t.Errorf("Expected 3 pending, got %d", p.Pending())
	}

	t.Logf("Buffer full test passed: %d pending, %d dropped", p.Pending(), p.dropped)
}

func TestPipelineNilPool(t *testing.T) {
	// Pipeline should work without DB (RNF02.1)
	p := NewPipeline(nil, 100)
	p.Start()

	p.Send(Entry{
		QueriedAt: time.Now(),
		ClientIP:  net.ParseIP("10.0.0.1"),
		GroupID:   1,
		Domain:    "no-db.com.",
		QueryType: 1,
		Action:    ActionAllowed,
	})

	time.Sleep(2 * time.Second)
	p.Stop()

	// Should not crash — entries discarded silently
	t.Log("Pipeline works without database connection")
}