package store

import (
	"context"
	"fmt"
)

// BlocklistEntry represents a domain from the database.
type BlocklistEntry struct {
	Domain   string
	ListType int // 0=blacklist, 1=whitelist
}

// LoadActiveBlocklistEntries loads all domains from active blocklists.
// Returns them split into blacklist and whitelist entries.
// This is called at startup and on LISTEN/NOTIFY reload.
func (s *Store) LoadActiveBlocklistEntries(ctx context.Context) (blacklist []string, whitelist []string, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT be.domain, bl.list_type
		FROM blocklist_entries be
		JOIN blocklists bl ON bl.id = be.list_id
		WHERE bl.active = true
		  AND bl.id NOT IN (SELECT list_id FROM blocklist_categories)
		ORDER BY bl.list_type, be.domain
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("store: load blocklist entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var domain string
		var listType int

		if err := rows.Scan(&domain, &listType); err != nil {
			return nil, nil, fmt.Errorf("store: scan blocklist entry: %w", err)
		}

		switch listType {
		case 0:
			blacklist = append(blacklist, domain)
		case 1:
			whitelist = append(whitelist, domain)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("store: iterate blocklist entries: %w", err)
	}

	return blacklist, whitelist, nil
}

// InsertBlocklist creates a new blocklist and returns its ID.
func (s *Store) InsertBlocklist(ctx context.Context, name string, sourceURL string, listType int) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO blocklists (name, source_url, list_type)
		VALUES ($1, $2, $3)
		RETURNING id
	`, name, sourceURL, listType).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: insert blocklist: %w", err)
	}
	return id, nil
}

// InsertBlocklistEntries bulk-inserts domains into a blocklist.
// Uses a batch approach for performance.
func (s *Store) InsertBlocklistEntries(ctx context.Context, listID int, domains []string) (int, error) {
	if len(domains) == 0 {
		return 0, nil
	}

	// Use COPY for bulk insert — much faster than individual INSERTs
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	count := 0
	for _, domain := range domains {
		_, err := tx.Exec(ctx, `
			INSERT INTO blocklist_entries (list_id, domain)
			VALUES ($1, $2)
			ON CONFLICT (list_id, domain) DO NOTHING
		`, listID, domain)
		if err != nil {
			return count, fmt.Errorf("store: insert entry %s: %w", domain, err)
		}
		count++
	}

	// Update domain count on the blocklist
	_, err = tx.Exec(ctx, `
		UPDATE blocklists SET domain_count = $1, updated_at = NOW() WHERE id = $2
	`, count, listID)
	if err != nil {
		return count, fmt.Errorf("store: update blocklist count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("store: commit: %w", err)
	}

	return count, nil
}
