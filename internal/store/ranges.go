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