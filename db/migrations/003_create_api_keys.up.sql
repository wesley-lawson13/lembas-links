CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key         VARCHAR(255) UNIQUE NOT NULL,
    name        VARCHAR(255),
    created_at  TIMESTAMP DEFAULT NOW()
);
