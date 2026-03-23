# CLAUDE.md — DNS Filter Empresarial

## Project

Enterprise DNS filter for 500-5000 device networks. Built from scratch in Go with miekg/dns.
Single binary: DNS server (:53) + REST API (HTTPS) + Captive Portal (HTTP :80).
UI in Portuguese (BR). Reference implementation: Blocky (github.com/0xERR0R/blocky) — read, never copy.

## Stack

- Go + github.com/miekg/dns (DNS from scratch)
- PostgreSQL + TimescaleDB (logs + config)
- Redis (DNS cache L2 + identity sessions)
- React + Vite + TypeScript + TanStack Table + Shadcn/ui
- golang-migrate for DB migrations

## Commands

```bash
go build ./cmd/dnsfilter          # build
go test ./...                      # run all tests
go test ./internal/dns/...         # test single package
go run ./cmd/dnsfilter             # run server
docker-compose up -d               # start PostgreSQL + Redis (dev)
docker-compose down                # stop dev services
```

## Architecture — DNS Hot Path

```
Query :53 → Listener → Identity Resolver (in-memory)
  → unknown IP + captive mode? → hijack to Captive Portal
  → known IP → Cache (L1 mem → L2 Redis)
    → Policy Engine (in-memory, ALWAYS runs, even on cache hit)
      → blocked → return block page IP
      → allowed + hit → respond from cache
      → allowed + miss → Upstream → write cache → respond
  → fire log event to async channel (never blocks)
```

## Hard Rules

- Identity Resolver, Policy Engine, blacklists: EXCLUSIVELY in-memory. Zero I/O on hot path.
- Log pipeline: async channel (100k buffer). If full, discard silently. NEVER block DNS.
- DNS server MUST keep working if PostgreSQL or Redis are offline.
- Whitelist ALWAYS beats blacklist. Policy Engine ALWAYS runs (even on cache hit).
- Cache stores DNS responses only. Filtering decisions are per-query.
- Config reload via PostgreSQL LISTEN/NOTIFY. No restart needed.

## Key Schemas

Log table: no UUID PK, SMALLINT enums, daily partitions (TimescaleDB hypertable), trigram index on domain.
Identity: `ip_ranges` (cidr, group_id, auth_mode) + `active_sessions` (client_ip PK, user_id, group_id, expires_at). Both mirrored in-memory.
Full schema: see docs/REQUIREMENTS.md

## Code Conventions

- Public interfaces must have tests
- Error wrapping: `fmt.Errorf("module: context: %w", err)`
- Structured service logs with levels (debug/info/warn/error)
- Variable names, structs, functions: English
- UI messages, business comments: Portuguese (BR)
- Each `internal/` package must be testable in isolation — depend on interfaces, not concrete types
- HTTP handlers receive interfaces, never concrete DB structs

## Git Workflow

- Branches: `main` (stable) ← `develop` (integration) ← `feat/...`, `fix/...`, `docs/...`
- Commit format: Conventional Commits in English
  - `feat(dns): add UDP listener on port 53`
  - `fix(cache): respect TTL floor configuration`
  - `test(filter): add blacklist lookup benchmarks`
  - `refactor(identity): extract CIDR matcher to separate file`
  - `docs: update phase 2 architecture notes`
  - `chore: add docker-compose for dev environment`
- Commits must be atomic: one logical change per commit
- Never commit code that doesn't compile
- Tag releases per phase: `v0.1.0` (phase 1), `v0.2.0` (phase 2), etc.

## Project Structure

```
cmd/dnsfilter/main.go          # entry point
internal/dns/                   # UDP/TCP listener, handler, upstream resolver
internal/filter/                # policy engine, blacklist, whitelist, categories
internal/cache/                 # L1 memory + L2 Redis
internal/identity/              # IP→user→group resolver, sessions, CIDR ranges
internal/captive/               # HTTP login portal
internal/logging/               # async pipeline, batch writer, retention
internal/api/                   # REST API, JWT auth, middleware
internal/config/                # YAML loader, defaults
internal/store/                 # PostgreSQL queries, migrations
web/                            # React frontend
migrations/                     # SQL files (golang-migrate)
docs/REQUIREMENTS.md            # Full requirements (RF01-RF10, RNF01-RNF07)
```

## Out of Scope (v1)

DoT/DoH, AD/LDAP, i18n, DNSSEC validation, mobile agents, billing — all Phase 2.

## Developer Context

Developer is learning Go and networking simultaneously. When generating code:
- Explain Go idioms when using them
- Show trade-offs when multiple approaches exist
- Flag performance concerns proactively
- Always specify file paths
- Read docs/REQUIREMENTS.md for detailed specs before suggesting implementations
