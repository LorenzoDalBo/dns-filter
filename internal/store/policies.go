package store

import (
	"context"
	"fmt"
)

// Policy maps a group to its blocked categories.
type Policy struct {
	ID         int   `json:"id"`
	GroupID    int   `json:"group_id"`
	Categories []int `json:"blocked_categories"`
}

// GetGroupBlockedCategories returns category IDs blocked for a group (RF03.4).
func (s *Store) GetGroupBlockedCategories(ctx context.Context, groupID int) ([]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT category_id FROM policies WHERE group_id = $1
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("store: get policies: %w", err)
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

// SetGroupPolicy replaces all blocked categories for a group (RF03.4).
func (s *Store) SetGroupPolicy(ctx context.Context, groupID int, categoryIDs []int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM policies WHERE group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("store: clear policies: %w", err)
	}

	for _, catID := range categoryIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO policies (group_id, category_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, groupID, catID)
		if err != nil {
			return fmt.Errorf("store: insert policy: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ListCategories returns all available categories.
func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, COALESCE(description, '') FROM categories ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list categories: %w", err)
	}
	defer rows.Close()

	var cats []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Description); err != nil {
			continue
		}
		cats = append(cats, c)
	}
	return cats, nil
}

type Category struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LoadCategoryDomains loads all domains grouped by category (RF03.3).
// Returns a map: categoryID → list of domains.
func (s *Store) LoadCategoryDomains(ctx context.Context) (map[int][]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT bc.category_id, be.domain
		FROM blocklist_categories bc
		JOIN blocklist_entries be ON be.list_id = bc.list_id
		JOIN blocklists bl ON bl.id = bc.list_id
		WHERE bl.active = true
	`)
	if err != nil {
		return nil, fmt.Errorf("store: load category domains: %w", err)
	}
	defer rows.Close()

	result := make(map[int][]string)
	for rows.Next() {
		var catID int
		var domain string
		if err := rows.Scan(&catID, &domain); err != nil {
			continue
		}
		result[catID] = append(result[catID], domain)
	}
	return result, nil
}

// LoadGroupPolicies loads all group → blocked categories mappings (RF03.4).
// Returns a map: groupID → set of blocked categoryIDs.
func (s *Store) LoadGroupPolicies(ctx context.Context) (map[int]map[int]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT group_id, category_id FROM policies`)
	if err != nil {
		return nil, fmt.Errorf("store: load policies: %w", err)
	}
	defer rows.Close()

	result := make(map[int]map[int]bool)
	for rows.Next() {
		var groupID, catID int
		if err := rows.Scan(&groupID, &catID); err != nil {
			continue
		}
		if result[groupID] == nil {
			result[groupID] = make(map[int]bool)
		}
		result[groupID][catID] = true
	}
	return result, nil
}
