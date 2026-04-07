package models

import (
	"database/sql"
	"fmt"
	"time"
)

func (s *URLStore) GetSlug() (string, error) {

	var slug string

	query := `
        SELECT slug
        FROM quotes
        WHERE use_count = (SELECT MIN(use_count) FROM quotes)
        ORDER BY RANDOM()
        LIMIT 1
    `

	err := s.db.QueryRow(query).Scan(&slug)
	if err != nil {
		return "", fmt.Errorf("failed to get slug: %w", err)
	}

	return slug, nil
}

func (s *URLStore) CreateURL(slug, original, apiKey string) error {

	// for now give the urls an expiration time of 30 days.
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	query := `
        INSERT INTO urls (slug, original, api_key, expires_at)
        VALUES ($1, $2, $3, $4)
    `

	_, err := s.db.Exec(query, slug, original, apiKey, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create new url: %w", err)
	}

	return nil
}

func (s *URLStore) GetURL(slug string) (*URL, error) {

	url := &URL{}

	query := `
        SELECT id, slug, original, api_key, click_count, expires_at, created_at, is_active
        FROM urls
        WHERE slug = $1
    `

	err := s.db.QueryRow(query, slug).Scan(
		&url.ID,
		&url.Slug,
		&url.Original,
		&url.APIKey,
		&url.ClickCount,
		&url.ExpiresAt,
		&url.CreatedAt,
		&url.IsActive,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("slug not found: %s", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get url: %w", err)
	}

	return url, nil
}

func (s *URLStore) DeleteURL(slug string) error {

	query := `
        UPDATE urls
        SET is_active = FALSE
        WHERE slug = $1
    `

	result, err := s.db.Exec(query, slug)
	if err != nil {
		return fmt.Errorf("failed to delete url: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("slug not found: %s", slug)
	}

	return nil
}
