package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type LogEntry struct {
	QueriedAt  time.Time `json:"queried_at"`
	ClientIP   string    `json:"client_ip"`
	UserID     *int      `json:"user_id"`
	GroupID    int       `json:"group_id"`
	Domain     string    `json:"domain"`
	QueryType  int       `json:"query_type"`
	Action     int       `json:"action"`
	ResponseMs float32   `json:"response_ms"`
	Upstream   string    `json:"upstream"`
}

type LogFilter struct {
	ClientIP string
	Domain   string
	Action   string
	DateFrom string
	DateTo   string
	Limit    int
	Offset   int
}

// QueryLogs returns paginated log entries with filters (RF08.2, RF08.3).
func (s *Store) QueryLogs(ctx context.Context, f LogFilter) ([]LogEntry, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	argN := 1

	if f.ClientIP != "" {
		where = append(where, fmt.Sprintf("client_ip = $%d::inet", argN))
		args = append(args, f.ClientIP)
		argN++
	}
	if f.Domain != "" {
		where = append(where, fmt.Sprintf("domain ILIKE $%d", argN))
		args = append(args, "%"+f.Domain+"%")
		argN++
	}
	if f.Action != "" {
		where = append(where, fmt.Sprintf("action = $%d", argN))
		args = append(args, f.Action)
		argN++
	}
	if f.DateFrom != "" {
		where = append(where, fmt.Sprintf("queried_at >= $%d::timestamptz", argN))
		args = append(args, f.DateFrom)
		argN++
	}
	if f.DateTo != "" {
		where = append(where, fmt.Sprintf("queried_at <= $%d::timestamptz", argN))
		args = append(args, f.DateTo)
		argN++
	}

	whereClause := strings.Join(where, " AND ")

	// Count total for pagination
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM dns_query_logs WHERE %s", whereClause)
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store: count logs: %w", err)
	}

	// Fetch page
	dataQuery := fmt.Sprintf(`
		SELECT queried_at, client_ip::text, user_id, group_id, domain, query_type,
		       action, response_ms, COALESCE(upstream, '')
		FROM dns_query_logs
		WHERE %s
		ORDER BY queried_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argN, argN+1)

	args = append(args, f.Limit, f.Offset)

	rows, err := s.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: query logs: %w", err)
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var l LogEntry
		var clientIP string
		if err := rows.Scan(&l.QueriedAt, &clientIP, &l.UserID, &l.GroupID,
			&l.Domain, &l.QueryType, &l.Action, &l.ResponseMs, &l.Upstream); err != nil {
			return nil, 0, fmt.Errorf("store: scan log: %w", err)
		}
		l.ClientIP = clientIP
		logs = append(logs, l)
	}

	return logs, total, nil
}

// TopItem represents a domain or client with its count.
type TopItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// DashboardStats returns aggregated statistics for the dashboard (RF08.1).
type DashboardStats struct {
	TotalToday     int       `json:"total_today"`
	TotalWeek      int       `json:"total_week"`
	TotalMonth     int       `json:"total_month"`
	BlockedPercent float64   `json:"blocked_percent"`
	TopDomains     []TopItem `json:"top_domains"`
	TopBlocked     []TopItem `json:"top_blocked"`
	TopClients     []TopItem `json:"top_clients"`
}

// GetDashboardStats returns aggregated stats for the dashboard (RF08.1).
func (s *Store) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Total queries today, this week, this month
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE queried_at >= CURRENT_DATE) as today,
			COUNT(*) FILTER (WHERE queried_at >= CURRENT_DATE - INTERVAL '7 days') as week,
			COUNT(*) FILTER (WHERE queried_at >= CURRENT_DATE - INTERVAL '30 days') as month
		FROM dns_query_logs
		WHERE queried_at >= CURRENT_DATE - INTERVAL '30 days'
	`).Scan(&stats.TotalToday, &stats.TotalWeek, &stats.TotalMonth)
	if err != nil {
		return nil, fmt.Errorf("store: dashboard totals: %w", err)
	}

	// Blocked percentage (today)
	if stats.TotalToday > 0 {
		var blocked int
		err = s.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM dns_query_logs
			WHERE queried_at >= CURRENT_DATE AND action = 1
		`).Scan(&blocked)
		if err == nil {
			stats.BlockedPercent = float64(blocked) / float64(stats.TotalToday) * 100
		}
	}

	// Top 10 domains (today)
	stats.TopDomains, _ = s.queryTopItems(ctx, `
		SELECT domain, COUNT(*) as cnt FROM dns_query_logs
		WHERE queried_at >= CURRENT_DATE
		GROUP BY domain ORDER BY cnt DESC LIMIT 10
	`)

	// Top 10 blocked domains (today)
	stats.TopBlocked, _ = s.queryTopItems(ctx, `
		SELECT domain, COUNT(*) as cnt FROM dns_query_logs
		WHERE queried_at >= CURRENT_DATE AND action = 1
		GROUP BY domain ORDER BY cnt DESC LIMIT 10
	`)

	// Top 10 clients by volume (today)
	stats.TopClients, _ = s.queryTopItems(ctx, `
		SELECT client_ip::text, COUNT(*) as cnt FROM dns_query_logs
		WHERE queried_at >= CURRENT_DATE
		GROUP BY client_ip ORDER BY cnt DESC LIMIT 10
	`)

	return stats, nil
}

func (s *Store) queryTopItems(ctx context.Context, query string) ([]TopItem, error) {
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TopItem
	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}
