package filter

import "fmt"

// Action represents the filtering decision.
type Action int

const (
	ActionAllow Action = iota
	ActionBlock
)

func (a Action) String() string {
	switch a {
	case ActionAllow:
		return "ALLOW"
	case ActionBlock:
		return "BLOCK"
	default:
		return "UNKNOWN"
	}
}

// Result holds the filtering decision and reason.
type Result struct {
	Action Action
	Reason string // why it was blocked (empty if allowed)
}

// Engine evaluates DNS queries against blacklist and whitelist (RF03.1).
// Whitelist always takes priority over blacklist (RF03.5).
type Engine struct {
	blacklist *Blacklist
	whitelist *Blacklist // reuse same structure — it's just a set of domains
}

func NewEngine(blacklist *Blacklist, whitelist *Blacklist) *Engine {
	return &Engine{
		blacklist: blacklist,
		whitelist: whitelist,
	}
}

// Evaluate checks a domain against whitelist then blacklist.
// Called for EVERY query, even on cache hit (RF03.1).
func (e *Engine) Evaluate(domain string) Result {
	// Whitelist has priority (RF03.5)
	if e.whitelist != nil && e.whitelist.Contains(domain) {
		return Result{Action: ActionAllow, Reason: "whitelist"}
	}

	// Check blacklist
	if e.blacklist != nil && e.blacklist.Contains(domain) {
		fmt.Printf("  → BLOCKED: %s (blacklist)\n", domain)
		return Result{Action: ActionBlock, Reason: "blacklist"}
	}

	return Result{Action: ActionAllow}
}
