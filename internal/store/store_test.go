package store

import (
	"context"
	"os"
	"testing"
)

func getTestDatabaseURL() string {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://dnsfilter:dnsfilter123@localhost:5432/dnsfilter?sslmode=disable"
	}
	return url
}

func TestStoreConnect(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, getTestDatabaseURL())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer s.Close()

	t.Log("Connected to PostgreSQL successfully")
}

func TestBlocklistRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, getTestDatabaseURL())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer s.Close()

	// Clean up from previous test runs
	s.pool.Exec(ctx, "DELETE FROM blocklist_entries WHERE list_id IN (SELECT id FROM blocklists WHERE name = 'test-list')")
	s.pool.Exec(ctx, "DELETE FROM blocklists WHERE name = 'test-list'")

	// Create a test blocklist
	listID, err := s.InsertBlocklist(ctx, "test-list", "", 0) // 0=blacklist
	if err != nil {
		t.Fatalf("InsertBlocklist failed: %v", err)
	}
	t.Logf("Created blocklist with ID %d", listID)

	// Insert test domains
	domains := []string{"ads.test.com", "tracker.test.com", "malware.test.com"}
	count, err := s.InsertBlocklistEntries(ctx, listID, domains)
	if err != nil {
		t.Fatalf("InsertBlocklistEntries failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 entries, got %d", count)
	}

	// Load and verify
	blacklist, whitelist, err := s.LoadActiveBlocklistEntries(ctx)
	if err != nil {
		t.Fatalf("LoadActiveBlocklistEntries failed: %v", err)
	}

	if len(whitelist) != 0 {
		t.Errorf("Expected 0 whitelist entries, got %d", len(whitelist))
	}

	// Check our test domains are in the blacklist
	found := 0
	for _, d := range blacklist {
		for _, expected := range domains {
			if d == expected {
				found++
			}
		}
	}
	if found != 3 {
		t.Errorf("Expected 3 test domains in blacklist, found %d (total blacklist: %d)", found, len(blacklist))
	}

	t.Logf("Loaded %d blacklist entries, %d whitelist entries", len(blacklist), len(whitelist))

	// Clean up
	s.pool.Exec(ctx, "DELETE FROM blocklist_entries WHERE list_id = $1", listID)
	s.pool.Exec(ctx, "DELETE FROM blocklists WHERE id = $1", listID)
}
