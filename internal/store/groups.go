package store

import (
	"context"
	"fmt"
)

type Group struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Store) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, COALESCE(description, '') FROM groups ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description); err != nil {
			return nil, fmt.Errorf("store: scan group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func (s *Store) CreateGroup(ctx context.Context, name, description string) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO groups (name, description) VALUES ($1, $2) RETURNING id
	`, name, description).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: create group: %w", err)
	}
	return id, nil
}

// UpdateGroup updates a group's name and description.
func (s *Store) UpdateGroup(ctx context.Context, id int, name, description string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE groups SET name = $1, description = $2 WHERE id = $3
	`, name, description, id)
	if err != nil {
		return fmt.Errorf("store: update group: %w", err)
	}
	return nil
}

// DeleteGroup removes a group.
func (s *Store) DeleteGroup(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM groups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete group: %w", err)
	}
	return nil
}
