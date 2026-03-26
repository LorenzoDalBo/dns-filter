CREATE TABLE blocklists (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    source_url  TEXT,
    list_type   SMALLINT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT true,
    domain_count INTEGER NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON COLUMN blocklists.list_type IS '0=blacklist, 1=whitelist';
COMMENT ON COLUMN blocklists.source_url IS 'NULL for manual lists, URL for external lists';

CREATE TABLE blocklist_entries (
    id          SERIAL PRIMARY KEY,
    list_id     INTEGER NOT NULL REFERENCES blocklists(id) ON DELETE CASCADE,
    domain      VARCHAR(253) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_blocklist_entry ON blocklist_entries (list_id, domain);
CREATE INDEX idx_blocklist_entries_domain ON blocklist_entries (domain);

CREATE TABLE blocklist_categories (
    list_id     INTEGER NOT NULL REFERENCES blocklists(id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (list_id, category_id)
);