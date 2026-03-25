package identity

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// AuthMode defines how a client authenticates.
type AuthMode int

const (
	AuthNone          AuthMode = 0 // Policy applied directly by IP/CIDR
	AuthCaptivePortal AuthMode = 1 // Login required
)

// Identity holds the resolved client information.
type Identity struct {
	UserID   int
	Username string
	GroupID  int
	AuthMode AuthMode
}

// IPRange represents a CIDR range mapped to a group.
type IPRange struct {
	Network  *net.IPNet
	GroupID  int
	AuthMode AuthMode
}

// Session represents an authenticated captive portal session.
type Session struct {
	ClientIP  net.IP
	UserID    int
	Username  string
	GroupID   int
	ExpiresAt time.Time
}

// Resolver maps client IPs to identities using in-memory data (RF05.8).
// Lookup priority:
//  1. Active session → use session's group
//  2. IP range with auth_mode=none → use range's group
//  3. IP range with auth_mode=captive_portal → require login
//  4. No match → default policy
type Resolver struct {
	mu             sync.RWMutex
	sessions       map[string]*Session // key: IP string
	ranges         []IPRange
	defaultGroupID int
	defaultAuth    AuthMode
}

func NewResolver(defaultGroupID int) *Resolver {
	return &Resolver{
		sessions:       make(map[string]*Session),
		ranges:         make([]IPRange, 0),
		defaultGroupID: defaultGroupID,
		defaultAuth:    AuthNone,
	}
}

// Resolve looks up the identity for a client IP.
// This is called on EVERY DNS query and must be fast (RF05.8).
func (r *Resolver) Resolve(clientIP net.IP) (*Identity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ipStr := clientIP.String()

	// Priority 1: Active session
	if session, ok := r.sessions[ipStr]; ok {
		if time.Now().Before(session.ExpiresAt) {
			return &Identity{
				UserID:   session.UserID,
				Username: session.Username,
				GroupID:  session.GroupID,
				AuthMode: AuthNone, // already authenticated
			}, nil
		}
		// Session expired — will be cleaned up by evictor
	}

	// Priority 2-3: IP range match
	for _, ipRange := range r.ranges {
		if ipRange.Network.Contains(clientIP) {
			return &Identity{
				GroupID:  ipRange.GroupID,
				AuthMode: ipRange.AuthMode,
			}, nil
		}
	}

	// Priority 4: Default policy (RF05.7)
	return &Identity{
		GroupID:  r.defaultGroupID,
		AuthMode: r.defaultAuth,
	}, nil
}

// AddSession registers an authenticated session (RF06.3).
// Called after successful captive portal login.
func (r *Resolver) AddSession(session *Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ipStr := session.ClientIP.String()
	r.sessions[ipStr] = session
	fmt.Printf("Identity: session created for %s (user=%s, group=%d, expires=%s)\n",
		ipStr, session.Username, session.GroupID,
		session.ExpiresAt.Format("15:04:05"))
}

// RemoveSession deletes a session by IP.
func (r *Resolver) RemoveSession(clientIP net.IP) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, clientIP.String())
}

// LoadRanges replaces all IP ranges in memory.
// Called at startup and on LISTEN/NOTIFY reload (RF03.9).
func (r *Resolver) LoadRanges(ranges []IPRange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ranges = ranges
	fmt.Printf("Identity: loaded %d IP ranges\n", len(ranges))
}

// AddRange appends a single IP range without replacing existing ones.
// Used when creating a new range via API.
func (r *Resolver) AddRange(ipRange IPRange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ranges = append(r.ranges, ipRange)
	fmt.Printf("Identity: added range %s (group=%d, auth=%d), total=%d\n",
		ipRange.Network.String(), ipRange.GroupID, ipRange.AuthMode, len(r.ranges))
}

// SessionCount returns number of active sessions.
func (r *Resolver) SessionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

// EvictExpiredSessions removes sessions past their TTL (RF05.5).
// Called periodically by a background goroutine.
func (r *Resolver) EvictExpiredSessions() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	removed := 0

	for ip, session := range r.sessions {
		if now.After(session.ExpiresAt) {
			delete(r.sessions, ip)
			removed++
		}
	}

	return removed
}

// StartSessionEvictor runs a background goroutine that cleans up
// expired sessions every minute.
func (r *Resolver) StartSessionEvictor() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if removed := r.EvictExpiredSessions(); removed > 0 {
				fmt.Printf("Identity: evicted %d expired sessions (%d remaining)\n",
					removed, r.SessionCount())
			}
		}
	}()
}
