package identity

import (
	"net"
	"testing"
	"time"
)

func TestResolveSession(t *testing.T) {
	r := NewResolver(1) // default group = 1

	clientIP := net.ParseIP("192.168.1.50")

	// Before session: should return default
	id, err := r.Resolve(clientIP)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if id.GroupID != 1 {
		t.Errorf("Expected default group 1, got %d", id.GroupID)
	}

	// Add session
	r.AddSession(&Session{
		ClientIP:  clientIP,
		UserID:    10,
		Username:  "joao",
		GroupID:   3,
		ExpiresAt: time.Now().Add(8 * time.Hour),
	})

	// After session: should return session's group
	id, err = r.Resolve(clientIP)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if id.GroupID != 3 {
		t.Errorf("Expected session group 3, got %d", id.GroupID)
	}
	if id.Username != "joao" {
		t.Errorf("Expected username joao, got %s", id.Username)
	}
}

func TestResolveIPRange(t *testing.T) {
	r := NewResolver(1)

	_, devNet, _ := net.ParseCIDR("10.0.1.0/24")
	_, guestNet, _ := net.ParseCIDR("10.0.5.0/24")

	r.LoadRanges([]IPRange{
		{Network: devNet, GroupID: 2, AuthMode: AuthNone},
		{Network: guestNet, GroupID: 4, AuthMode: AuthCaptivePortal},
	})

	// IP in dev range
	id, _ := r.Resolve(net.ParseIP("10.0.1.100"))
	if id.GroupID != 2 {
		t.Errorf("Expected dev group 2, got %d", id.GroupID)
	}
	if id.AuthMode != AuthNone {
		t.Error("Dev range should be AuthNone")
	}

	// IP in guest range
	id, _ = r.Resolve(net.ParseIP("10.0.5.50"))
	if id.GroupID != 4 {
		t.Errorf("Expected guest group 4, got %d", id.GroupID)
	}
	if id.AuthMode != AuthCaptivePortal {
		t.Error("Guest range should be AuthCaptivePortal")
	}

	// IP not in any range — default
	id, _ = r.Resolve(net.ParseIP("172.16.0.1"))
	if id.GroupID != 1 {
		t.Errorf("Expected default group 1, got %d", id.GroupID)
	}
}

func TestSessionPriorityOverRange(t *testing.T) {
	r := NewResolver(1)

	_, guestNet, _ := net.ParseCIDR("10.0.5.0/24")
	r.LoadRanges([]IPRange{
		{Network: guestNet, GroupID: 4, AuthMode: AuthCaptivePortal},
	})

	clientIP := net.ParseIP("10.0.5.50")

	// Before login: should require captive portal
	id, _ := r.Resolve(clientIP)
	if id.AuthMode != AuthCaptivePortal {
		t.Error("Should require captive portal before login")
	}

	// After login: session takes priority
	r.AddSession(&Session{
		ClientIP:  clientIP,
		UserID:    20,
		Username:  "visitante",
		GroupID:   5,
		ExpiresAt: time.Now().Add(8 * time.Hour),
	})

	id, _ = r.Resolve(clientIP)
	if id.GroupID != 5 {
		t.Errorf("Session should override range, expected group 5, got %d", id.GroupID)
	}
	if id.AuthMode != AuthNone {
		t.Error("Authenticated session should be AuthNone")
	}
}

func TestSessionExpiration(t *testing.T) {
	r := NewResolver(1)

	clientIP := net.ParseIP("192.168.1.50")

	// Add expired session
	r.AddSession(&Session{
		ClientIP:  clientIP,
		UserID:    10,
		Username:  "expired",
		GroupID:   3,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // already expired
	})

	// Should fall through to default because session is expired
	id, _ := r.Resolve(clientIP)
	if id.GroupID != 1 {
		t.Errorf("Expired session should fall to default, got group %d", id.GroupID)
	}

	// Evictor should clean it up
	removed := r.EvictExpiredSessions()
	if removed != 1 {
		t.Errorf("Expected 1 evicted, got %d", removed)
	}
	if r.SessionCount() != 0 {
		t.Errorf("Expected 0 sessions after eviction, got %d", r.SessionCount())
	}
}
