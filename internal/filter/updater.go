package filter

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Updater periodically downloads external blocklists (RF04.3).
type Updater struct {
	pool       *pgxpool.Pool
	downloader *Downloader
	interval   time.Duration
}

func NewUpdater(pool *pgxpool.Pool, interval time.Duration) *Updater {
	return &Updater{
		pool:       pool,
		downloader: NewDownloader(),
		interval:   interval,
	}
}

// Start launches the background updater goroutine.
func (u *Updater) Start() {
	// Run once at startup
	u.updateAll()

	go func() {
		ticker := time.NewTicker(u.interval)
		defer ticker.Stop()

		for range ticker.C {
			u.updateAll()
		}
	}()

	fmt.Printf("List updater: started (interval=%v)\n", u.interval)
}

// updateAll fetches all external lists that have a source_url.
func (u *Updater) updateAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rows, err := u.pool.Query(ctx, `
		SELECT id, name, source_url FROM blocklists
		WHERE source_url != '' AND source_url IS NOT NULL AND active = true
	`)
	if err != nil {
		fmt.Printf("List updater: query failed: %v\n", err)
		return
	}
	defer rows.Close()

	type listInfo struct {
		ID        int
		Name      string
		SourceURL string
	}

	var lists []listInfo
	for rows.Next() {
		var l listInfo
		if err := rows.Scan(&l.ID, &l.Name, &l.SourceURL); err != nil {
			fmt.Printf("List updater: scan failed: %v\n", err)
			continue
		}
		lists = append(lists, l)
	}

	for _, l := range lists {
		u.updateList(ctx, l.ID, l.Name, l.SourceURL)
	}
}

func (u *Updater) updateList(ctx context.Context, listID int, name, sourceURL string) {
	fmt.Printf("List updater: downloading %s (%s)...\n", name, sourceURL)

	domains, err := u.downloader.FetchDomains(ctx, sourceURL)
	if err != nil {
		fmt.Printf("List updater: %s failed: %v\n", name, err)
		return
	}

	if len(domains) == 0 {
		fmt.Printf("List updater: %s returned 0 domains, skipping\n", name)
		return
	}

	// Clear existing entries and re-insert
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		fmt.Printf("List updater: tx begin failed: %v\n", err)
		return
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM blocklist_entries WHERE list_id = $1`, listID)
	if err != nil {
		fmt.Printf("List updater: clear entries failed: %v\n", err)
		return
	}

	count := 0
	for _, domain := range domains {
		_, err := tx.Exec(ctx, `
			INSERT INTO blocklist_entries (list_id, domain)
			VALUES ($1, $2) ON CONFLICT (list_id, domain) DO NOTHING
		`, listID, domain)
		if err == nil {
			count++
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE blocklists SET domain_count = $1, updated_at = NOW() WHERE id = $2
	`, count, listID)
	if err != nil {
		fmt.Printf("List updater: update count failed: %v\n", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Printf("List updater: commit failed: %v\n", err)
		return
	}

	fmt.Printf("List updater: %s — %d domínios carregados\n", name, count)
}
