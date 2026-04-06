package models

import (
    "database/sql"
    "fmt"
)

func (s *URLStore) GetStats(slug string) (*URLStats, error) {

	urlStats := &URLStats{}

	query := `
        SELECT slug, original, click_count, expires_at, created_at, is_active
        FROM urls
        WHERE slug = $1
    `

	err := s.db.QueryRow(query, slug).Scan(
		&urlStats.Slug,
		&urlStats.Original,
		&urlStats.ClickCount,
		&urlStats.ExpiresAt,
		&urlStats.CreatedAt,
		&urlStats.IsActive,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("slug not found: %s", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get url: %w", err)
	}

	return urlStats, nil
}

func (s *URLStore) IncrementClickCount(slug string) error {

    query := `
        UPDATE quotes
        SET click_count = click_count + 1
        WHERE slug = $1
    `

    _, err := s.db.Exec(query, slug)
    if err != nil {
        return fmt.Errorf("failed to update click_count: %w", err)
    }

    return nil
}
