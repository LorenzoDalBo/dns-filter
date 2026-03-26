package filter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Downloader fetches external blocklists by URL (RF04.2, RF04.3).
type Downloader struct {
	client *http.Client
}

func NewDownloader() *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchDomains downloads a blocklist URL and parses domains from it.
// Supports hosts file format and plain domain format (RF04.4).
func (d *Downloader) FetchDomains(ctx context.Context, url string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("downloader: create request: %w", err)
	}

	req.Header.Set("User-Agent", "DNSFilter/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloader: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloader: %s returned status %d", url, resp.StatusCode)
	}

	// Limit to 50MB to prevent abuse
	limited := io.LimitReader(resp.Body, 50*1024*1024)

	var domains []string
	scanner := bufio.NewScanner(limited)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domain := parseLine(line)
		if domain != "" {
			domains = append(domains, domain)
		}
	}

	if err := scanner.Err(); err != nil {
		return domains, fmt.Errorf("downloader: read body: %w", err)
	}

	return domains, nil
}