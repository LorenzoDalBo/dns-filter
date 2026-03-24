CREATE TABLE ip_ranges (
    id          SERIAL PRIMARY KEY,
    cidr        CIDR NOT NULL,
    group_id    INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    auth_mode   SMALLINT NOT NULL DEFAULT 0,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON COLUMN ip_ranges.auth_mode IS '0=none (policy direct), 1=captive_portal';

CREATE TABLE active_sessions (
    client_ip   INET PRIMARY KEY,
    user_id     INTEGER,
    group_id    INTEGER NOT NULL REFERENCES groups(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL
);