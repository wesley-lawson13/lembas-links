CREATE TABLE urls (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        VARCHAR(255) UNIQUE NOT NULL,
    original    TEXT NOT NULL,
    api_key     VARCHAR(255),
    click_count INTEGER DEFAULT 0,
    expires_at  TIMESTAMP,
    created_at  TIMESTAMP DEFAULT NOW(),
    is_active   BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_urls_slug ON urls(slug);
