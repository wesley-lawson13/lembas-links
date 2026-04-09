package models

import (
	"fmt"
	"time"
)

type Click struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	ClickedAt time.Time `json:"clicked_at"`
	Referrer  string    `json:"referrer"`
	UserAgent string    `json:"user_agent"`
	IPAddress string    `json:"ip_address"`
}

func (s *URLStore) RecordClick(slug, referrer, userAgent, ipAddress string) error {

	query := `
        INSERT INTO clicks (slug, referrer, user_agent, ip_address)
        VALUES ($1, $2, $3, $4)
    `

	_, err := s.db.Exec(query, slug, referrer, userAgent, ipAddress)
	if err != nil {
		return fmt.Errorf("failed to record click: %w", err)
	}

	return nil
}

func (s *URLStore) GetClicks(slug string) ([]Click, error) {

	query := `
        SELECT id, slug, clicked_at, referrer, user_agent, ip_address
        FROM clicks
        WHERE slug = $1
        ORDER BY clicked_at DESC
        LIMIT 10
    `
	rows, err := s.db.Query(query, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get clicks: %w", err)
	}
	defer rows.Close()

	var clicks []Click
	for rows.Next() {

		var click Click
		err := rows.Scan(
			&click.ID,
			&click.Slug,
			&click.ClickedAt,
			&click.Referrer,
			&click.UserAgent,
			&click.IPAddress,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan click: %w", err)
		}

		clicks = append(clicks, click)
	}

	return clicks, nil
}
