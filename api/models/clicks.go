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

// DailyClickCount is one day's click total for a slug, Date formatted "2006-01-02".
type DailyClickCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// GetDailyClicks returns per-day click totals for the last days days, including
// zero-click days; called from handlers.GetStats.
func (s *URLStore) GetDailyClicks(slug string, days int) ([]DailyClickCount, error) {

	query := `
        SELECT d.day::date, COALESCE(c.count, 0)
        FROM generate_series(CURRENT_DATE - ($2 - 1) * INTERVAL '1 day', CURRENT_DATE, '1 day') AS d(day)
        LEFT JOIN (
            SELECT clicked_at::date AS day, COUNT(*) AS count
            FROM clicks
            WHERE slug = $1 AND clicked_at >= CURRENT_DATE - ($2 - 1) * INTERVAL '1 day'
            GROUP BY 1
        ) c ON c.day = d.day::date
        ORDER BY d.day ASC
    `
	rows, err := s.db.Query(query, slug, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily clicks: %w", err)
	}
	defer rows.Close()

	dailyClicks := make([]DailyClickCount, 0, days)
	for rows.Next() {

		var day time.Time
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			return nil, fmt.Errorf("failed to scan daily click count: %w", err)
		}

		dailyClicks = append(dailyClicks, DailyClickCount{
			Date:  day.Format("2006-01-02"),
			Count: count,
		})
	}

	return dailyClicks, nil
}

func (s *URLStore) GetClicks(slug string, limit int) ([]Click, error) {

	query := `
        SELECT id, slug, clicked_at, referrer, user_agent, ip_address
        FROM clicks
        WHERE slug = $1
        ORDER BY clicked_at DESC
        LIMIT $2
    `
	rows, err := s.db.Query(query, slug, limit)
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
