package filter

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Blacklist holds blocked domains in a hashmap for O(1) lookup (RF03.8).
// Supports exact match and wildcard blocking (RF03.6).
type Blacklist struct {
	mu      sync.RWMutex
	domains map[string]bool // exact domain matches
}

func NewBlacklist() *Blacklist {
	return &Blacklist{
		domains: make(map[string]bool),
	}
}

// Add registers a domain as blocked.
// Normalizes to lowercase with trailing dot (FQDN format).
func (b *Blacklist) Add(domain string) {
	domain = normalize(domain)
	b.mu.Lock()
	b.domains[domain] = true
	b.mu.Unlock()
}

// Remove unblocks a domain.
func (b *Blacklist) Remove(domain string) {
	domain = normalize(domain)
	b.mu.Lock()
	delete(b.domains, domain)
	b.mu.Unlock()
}

// Contains checks if a domain is blocked.
// Also checks parent domains to support wildcard blocking (RF03.6):
// if "example.com" is blocked, "ads.example.com" is also blocked.
func (b *Blacklist) Contains(domain string) bool {
	domain = normalize(domain)

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Check exact match first
	if b.domains[domain] {
		return true
	}

	// Walk up the domain tree for wildcard matching (RF03.6)
	// "sub.ads.example.com." → check "ads.example.com." → check "example.com."
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if b.domains[parent] {
			return true
		}
	}

	return false
}

// Size returns the number of domains in the blacklist.
func (b *Blacklist) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.domains)
}

// LoadFromFile reads domains from a file, one per line (RF04.4).
// Supports hosts file format (ignores 127.0.0.1/0.0.0.0 prefix)
// and plain domain format.
func (b *Blacklist) LoadFromFile(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("blacklist: open %s: %w", path, err)
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		domain := parseLine(line)
		if domain != "" {
			b.Add(domain)
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("blacklist: read %s: %w", path, err)
	}

	return count, nil
}

// parseLine extracts a domain from a line that may be in hosts file format.
// "127.0.0.1 ads.example.com" → "ads.example.com"
// "0.0.0.0 ads.example.com"   → "ads.example.com"
// "ads.example.com"           → "ads.example.com"
func parseLine(line string) string {
	// Remove inline comments
	if idx := strings.Index(line, "#"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}

	if line == "" {
		return ""
	}

	fields := strings.Fields(line)

	if len(fields) >= 2 {
		prefix := fields[0]
		// Hosts file format: skip the IP prefix
		if prefix == "127.0.0.1" || prefix == "0.0.0.0" || prefix == "::1" {
			return fields[1]
		}
	}

	// Plain domain format
	if len(fields) == 1 {
		return fields[0]
	}

	return ""
}

// normalize converts a domain to lowercase FQDN with trailing dot.
func normalize(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	return domain
}
