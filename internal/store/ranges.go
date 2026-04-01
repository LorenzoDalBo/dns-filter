package store

import (
	"context"
	"fmt"
)

type IPRange struct {
	ID          int    `json:"id"`
	CIDR        string `json:"cidr"`
	GroupID     int    `json:"group_id"`
	AuthMode    int    `json:"auth_mode"`
	Description string `json:"description"`
}

type BlocklistInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	SourceURL   string `json:"source_url"`
	ListType    int    `json:"list_type"`
	Active      bool   `json:"active"`
	DomainCount int    `json:"domain_count"`
}

func (s *Store) ListIPRanges(ctx context.Context) ([]IPRange, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, cidr::text, group_id, auth_mode, COALESCE(description, '')
		FROM ip_ranges ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("store: list ranges: %w", err)
	}
	defer rows.Close()

	var ranges []IPRange
	for rows.Next() {
		var r IPRange
		if err := rows.Scan(&r.ID, &r.CIDR, &r.GroupID, &r.AuthMode, &r.Description); err != nil {
			return nil, fmt.Errorf("store: scan range: %w", err)
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

func (s *Store) CreateIPRange(ctx context.Context, cidr string, groupID, authMode int, description string) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO ip_ranges (cidr, group_id, auth_mode, description)
		VALUES ($1::cidr, $2, $3, $4) RETURNING id
	`, cidr, groupID, authMode, description).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: create range: %w", err)
	}
	return id, nil
}

func (s *Store) ListBlocklists(ctx context.Context) ([]BlocklistInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, COALESCE(source_url, ''), list_type, active, domain_count
		FROM blocklists ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("store: list blocklists: %w", err)
	}
	defer rows.Close()

	var lists []BlocklistInfo
	for rows.Next() {
		var l BlocklistInfo
		if err := rows.Scan(&l.ID, &l.Name, &l.SourceURL, &l.ListType, &l.Active, &l.DomainCount); err != nil {
			return nil, fmt.Errorf("store: scan blocklist: %w", err)
		}
		lists = append(lists, l)
	}
	return lists, nil
}

// UpdateIPRange updates a range's settings.
func (s *Store) UpdateIPRange(ctx context.Context, id int, cidr string, groupID, authMode int, description string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE ip_ranges SET cidr = $1::cidr, group_id = $2, auth_mode = $3, description = $4
		WHERE id = $5
	`, cidr, groupID, authMode, description, id)
	if err != nil {
		return fmt.Errorf("store: update range: %w", err)
	}
	return nil
}

// DeleteIPRange removes an IP range.
func (s *Store) DeleteIPRange(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM ip_ranges WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete range: %w", err)
	}
	return nil
}

// UpdateBlocklist updates a blocklist's settings.
func (s *Store) UpdateBlocklist(ctx context.Context, id int, name string, active bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE blocklists SET name = $1, active = $2, updated_at = NOW() WHERE id = $3
	`, name, active, id)
	if err != nil {
		return fmt.Errorf("store: update blocklist: %w", err)
	}
	return nil
}

// DeleteBlocklist removes a blocklist and its entries.
func (s *Store) DeleteBlocklist(ctx context.Context, id int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM blocklist_entries WHERE list_id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete entries: %w", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM blocklists WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete blocklist: %w", err)
	}

	return tx.Commit(ctx)
}

// SetBlocklistCategories associates a blocklist with categories (RF04.5).
func (s *Store) SetBlocklistCategories(ctx context.Context, listID int, categoryIDs []int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM blocklist_categories WHERE list_id = $1`, listID)
	if err != nil {
		return fmt.Errorf("store: clear categories: %w", err)
	}

	for _, catID := range categoryIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO blocklist_categories (list_id, category_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, listID, catID)
		if err != nil {
			return fmt.Errorf("store: insert category: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetBlocklistCategories returns category IDs for a blocklist.
func (s *Store) GetBlocklistCategories(ctx context.Context, listID int) ([]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT category_id FROM blocklist_categories WHERE list_id = $1
	`, listID)
	if err != nil {
		return nil, fmt.Errorf("store: get blocklist categories: %w", err)
	}
	defer rows.Close()

	var cats []int
	for rows.Next() {
		var catID int
		if err := rows.Scan(&catID); err != nil {
			continue
		}
		cats = append(cats, catID)
	}
	return cats, nil
}

// LoadIPRangesForIdentity loads all IP ranges for the identity resolver (startup).
func (s *Store) LoadIPRangesForIdentity(ctx context.Context) ([]IPRange, error) {
	return s.ListIPRanges(ctx)
}
