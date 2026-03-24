package filter

import (
	"os"
	"testing"
)

func TestBlacklistExactMatch(t *testing.T) {
	bl := NewBlacklist()
	bl.Add("ads.example.com")

	if !bl.Contains("ads.example.com") {
		t.Error("Should match exact domain")
	}
	if !bl.Contains("ads.example.com.") {
		t.Error("Should match with trailing dot")
	}
	if !bl.Contains("ADS.EXAMPLE.COM") {
		t.Error("Should be case-insensitive")
	}
	if bl.Contains("safe.example.com") {
		t.Error("Should not match different domain")
	}
}

func TestBlacklistWildcard(t *testing.T) {
	bl := NewBlacklist()
	bl.Add("example.com")

	// Subdomains should be blocked when parent is blocked (RF03.6)
	if !bl.Contains("ads.example.com") {
		t.Error("Subdomain should be blocked")
	}
	if !bl.Contains("deep.sub.example.com") {
		t.Error("Deep subdomain should be blocked")
	}
	if bl.Contains("notexample.com") {
		t.Error("Different domain should not match")
	}
}

func TestBlacklistLoadHostsFile(t *testing.T) {
	// Create a temp hosts file
	content := `# This is a comment
127.0.0.1 ads.example.com
0.0.0.0 tracker.evil.com
0.0.0.0 malware.bad.com # inline comment

# Another comment
pure-domain.com
`
	tmpFile, err := os.CreateTemp("", "blocklist-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(content)
	tmpFile.Close()

	bl := NewBlacklist()
	count, err := bl.LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if count != 4 {
		t.Errorf("Expected 4 domains loaded, got %d", count)
	}

	if !bl.Contains("ads.example.com") {
		t.Error("Should contain ads.example.com (hosts format)")
	}
	if !bl.Contains("tracker.evil.com") {
		t.Error("Should contain tracker.evil.com (hosts format)")
	}
	if !bl.Contains("pure-domain.com") {
		t.Error("Should contain pure-domain.com (plain format)")
	}
}

func TestEngineWhitelistPriority(t *testing.T) {
	bl := NewBlacklist()
	bl.Add("example.com")

	wl := NewBlacklist()
	wl.Add("safe.example.com")

	engine := NewEngine(bl, wl)

	// Blacklisted domain should be blocked
	result := engine.Evaluate("ads.example.com")
	if result.Action != ActionBlock {
		t.Error("ads.example.com should be blocked")
	}

	// Whitelisted domain should be allowed even though parent is blacklisted (RF03.5)
	result = engine.Evaluate("safe.example.com")
	if result.Action != ActionAllow {
		t.Error("safe.example.com should be allowed (whitelist priority)")
	}

	// Unknown domain should be allowed
	result = engine.Evaluate("google.com")
	if result.Action != ActionAllow {
		t.Error("google.com should be allowed")
	}
}

func TestEngineNilLists(t *testing.T) {
	// Engine should work with nil whitelist
	engine := NewEngine(NewBlacklist(), nil)

	result := engine.Evaluate("anything.com")
	if result.Action != ActionAllow {
		t.Error("Should allow when no lists match")
	}
}
