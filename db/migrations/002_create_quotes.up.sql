CREATE TABLE quotes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote       TEXT NOT NULL,
    character   VARCHAR(255),
    source      VARCHAR(255),
    created_at  TIMESTAMP DEFAULT NOW()
);
