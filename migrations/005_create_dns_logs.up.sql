CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE dns_query_logs (
    queried_at      TIMESTAMPTZ     NOT NULL,
    client_ip       INET            NOT NULL,
    user_id         INTEGER,
    group_id        INTEGER         NOT NULL,
    domain          TEXT            NOT NULL,
    query_type      SMALLINT        NOT NULL,
    action          SMALLINT        NOT NULL,
    block_reason    SMALLINT,
    category_id     SMALLINT,
    response_ip     INET,
    response_ms     REAL,
    upstream        TEXT
);

COMMENT ON COLUMN dns_query_logs.query_type IS 'DNS type: 1=A, 28=AAAA, 5=CNAME, 15=MX, etc';
COMMENT ON COLUMN dns_query_logs.action IS '0=allowed, 1=blocked, 2=cached';
COMMENT ON COLUMN dns_query_logs.block_reason IS 'NULL if allowed; 1=blacklist, 2=category, 3=policy';

SELECT create_hypertable('dns_query_logs', 'queried_at',
    chunk_time_interval => INTERVAL '1 day');

CREATE INDEX idx_logs_client_ip ON dns_query_logs (client_ip, queried_at DESC);
CREATE INDEX idx_logs_user_id   ON dns_query_logs (user_id, queried_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX idx_logs_domain    ON dns_query_logs USING gin (domain gin_trgm_ops);
CREATE INDEX idx_logs_action    ON dns_query_logs (action, queried_at DESC);