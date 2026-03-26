CREATE TABLE groups (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE categories (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE policies (
    id          SERIAL PRIMARY KEY,
    group_id    INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    UNIQUE(group_id, category_id)
);

COMMENT ON TABLE policies IS 'Each row means group X blocks category Y';

INSERT INTO groups (name, description) VALUES
    ('default', 'Política padrão para IPs não identificados');

INSERT INTO categories (name, description) VALUES
    ('malware', 'Sites de malware e phishing'),
    ('ads', 'Publicidade e rastreamento'),
    ('adult', 'Conteúdo adulto'),
    ('social', 'Redes sociais'),
    ('streaming', 'Serviços de streaming'),
    ('gaming', 'Sites de jogos');