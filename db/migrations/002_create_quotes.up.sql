CREATE TABLE quotes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote       TEXT NOT NULL,
    character   VARCHAR(255),
    source      VARCHAR(255),
    slug        VARCHAR(255) UNIQUE NOT NULL,
    use_count   INTEGER DEFAULT 0,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_quotes_slug ON quotes(slug);
CREATE INDEX idx_quotes_use_count ON quotes(use_count);
