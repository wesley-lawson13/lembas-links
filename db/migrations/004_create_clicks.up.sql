CREATE TABLE clicks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       VARCHAR(255) NOT NULL,
    clicked_at TIMESTAMP DEFAULT NOW(),
    referrer   VARCHAR(255),
    user_agent TEXT,
    ip_address VARCHAR(255)
);

CREATE INDEX idx_clicks_slug ON clicks(slug);
CREATE INDEX idx_clicks_clicked_at ON clicks(clicked_at);
