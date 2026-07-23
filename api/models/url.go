package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// slugForUseCount builds the public slug for the Nth use of a base quote slug:
// the clean base on the first use, "base-N" thereafter. Keeping this pure makes
// the reuse-counter formula testable without a database.
func slugForUseCount(base string, useCount int) string {
	if useCount <= 1 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, useCount)
}

// AllocateURL atomically claims the least-used base quote, bumps its use_count,
// and inserts a new url under a slug carrying a reuse counter (see
// slugForUseCount). The atomic UPDATE ... RETURNING hands each concurrent caller
// a distinct use_count for a given base, so generated slugs don't collide under
// normal operation; the bounded retry only guards the pathological case where a
// computed "base-N" happens to equal a different quote's clean slug. Because the
// base pool is finite but reusable this way, allocation never exhausts.
// Called from handlers.CreateLink, replacing the old GetSlug + CreateURL two-step.
func (s *URLStore) AllocateURL(original, apiKey string, expiresAt time.Time) (string, error) {

	const maxAttempts = 3

	for range maxAttempts {

		var base string
		var useCount int

		err := s.db.QueryRow(`
            UPDATE quotes
            SET use_count = use_count + 1
            WHERE id = (SELECT id FROM quotes ORDER BY use_count, RANDOM() LIMIT 1)
            RETURNING slug, use_count
        `).Scan(&base, &useCount)

		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no quotes available in pool")
		}
		if err != nil {
			return "", fmt.Errorf("failed to claim slug: %w", err)
		}

		slug := slugForUseCount(base, useCount)

		_, err = s.db.Exec(`
            INSERT INTO urls (slug, original, api_key, expires_at)
            VALUES ($1, $2, $3, $4)
        `, slug, original, apiKey, expiresAt)
		if err == nil {
			return slug, nil
		}

		// On a unique-violation the computed slug collided with an existing url;
		// retry with another quote. The already-bumped use_count on the losing
		// quote is left as-is (harmless: it just makes that base slightly less
		// preferred next time).
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			continue
		}
		return "", fmt.Errorf("failed to create url: %w", err)
	}

	return "", fmt.Errorf("failed to allocate a unique slug after %d attempts", maxAttempts)
}

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

func (s *URLStore) CreateURL(slug, original, apiKey string, expiresAt time.Time) error {

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

// ListURLsByAPIKey returns the caller's own active links, matched by hashing
// rawAPIKey and comparing against the stored hash in urls.api_key. Expired
// links are intentionally included: expiry stops redirects, not the owner's
// dashboard (the frontend flags them via expires_at); only owner deletion
// (is_active = FALSE) hides a link.
func (s *URLStore) ListURLsByAPIKey(rawAPIKey string) ([]URL, error) {

	query := `
        SELECT id, slug, original, api_key, click_count, expires_at, created_at, is_active
        FROM urls
        WHERE api_key = $1 AND is_active = TRUE
        ORDER BY created_at DESC
    `

	rows, err := s.db.Query(query, hashKey(rawAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to list urls: %w", err)
	}
	defer rows.Close()

	urls := []URL{}
	for rows.Next() {
		var url URL
		if err := rows.Scan(
			&url.ID,
			&url.Slug,
			&url.Original,
			&url.APIKey,
			&url.ClickCount,
			&url.ExpiresAt,
			&url.CreatedAt,
			&url.IsActive,
		); err != nil {
			return nil, fmt.Errorf("failed to scan url: %w", err)
		}
		urls = append(urls, url)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list urls: %w", err)
	}

	return urls, nil
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
