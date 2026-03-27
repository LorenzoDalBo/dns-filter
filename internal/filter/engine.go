package filter

import (
	"fmt"
	"sync"
)

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
	Action     Action
	Reason     string
	CategoryID int // 0 if not category-based
}

// Engine evaluates DNS queries against blacklist, whitelist and group policies (RF03.1).
// Whitelist always takes priority over blacklist (RF03.5).
type Engine struct {
	blacklist *Blacklist
	whitelist *Blacklist

	mu               sync.RWMutex
	domainCategories map[string]int       // domain → categoryID
	groupPolicies    map[int]map[int]bool // groupID → set of blocked categoryIDs
}

func NewEngine(blacklist *Blacklist, whitelist *Blacklist) *Engine {
	return &Engine{
		blacklist:        blacklist,
		whitelist:        whitelist,
		domainCategories: make(map[string]int),
		groupPolicies:    make(map[int]map[int]bool),
	}
}

// LoadCategories loads domain → category mappings into memory (RF03.3).
func (e *Engine) LoadCategories(catDomains map[int][]string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.domainCategories = make(map[string]int)
	total := 0
	for catID, domains := range catDomains {
		for _, domain := range domains {
			e.domainCategories[normalize(domain)] = catID
			total++
		}
	}
	fmt.Printf("Policy Engine: loaded %d domain-category mappings\n", total)
}

// LoadPolicies loads group → blocked categories mappings (RF03.4).
func (e *Engine) LoadPolicies(policies map[int]map[int]bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.groupPolicies = policies
	fmt.Printf("Policy Engine: loaded policies for %d groups\n", len(policies))
}

// Evaluate checks a domain against whitelist, blacklist, then group policy.
// Called for EVERY query, even on cache hit (RF03.1).
func (e *Engine) Evaluate(domain string) Result {
	return e.EvaluateForGroup(domain, 0)
}

// EvaluateForGroup checks a domain with group-specific policy (RF03.4).
func (e *Engine) EvaluateForGroup(domain string, groupID int) Result {
	// Whitelist has priority (RF03.5)
	if e.whitelist != nil && e.whitelist.Contains(domain) {
		return Result{Action: ActionAllow, Reason: "whitelist"}
	}

	// Check global blacklist
	if e.blacklist != nil && e.blacklist.Contains(domain) {
		fmt.Printf("  → BLOCKED: %s (blacklist)\n", domain)
		return Result{Action: ActionBlock, Reason: "blacklist"}
	}

	// Check group policy with categories (RF03.3, RF03.4)
	if groupID > 0 {
		e.mu.RLock()
		normalized := normalize(domain)
		catID, hasCat := e.domainCategories[normalized]
		if !hasCat {
			// Check parent domains for category
			catID, hasCat = e.findParentCategory(normalized)
		}

		if hasCat {
			blockedCats := e.groupPolicies[groupID]
			if blockedCats != nil && blockedCats[catID] {
				e.mu.RUnlock()
				fmt.Printf("  → BLOCKED: %s (category %d for group %d)\n", domain, catID, groupID)
				return Result{Action: ActionBlock, Reason: "category", CategoryID: catID}
			}
		}
		e.mu.RUnlock()
	}

	return Result{Action: ActionAllow}
}

// findParentCategory walks up the domain tree to find a category match.
func (e *Engine) findParentCategory(domain string) (int, bool) {
	parts := splitDomain(domain)
	for i := 1; i < len(parts)-1; i++ {
		parent := joinDomain(parts[i:])
		if catID, ok := e.domainCategories[parent]; ok {
			return catID, true
		}
	}
	return 0, false
}

func splitDomain(domain string) []string {
	result := []string{}
	current := ""
	for _, c := range domain {
		if c == '.' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func joinDomain(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "."
		}
		result += p
	}
	return result + "."
}
