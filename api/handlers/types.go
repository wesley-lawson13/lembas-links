package handlers

import "time"

// CreateLinkRequest is the request body for creating a short link.
type CreateLinkRequest struct {
	URL    string `json:"url"     example:"https://example.com/some/very/long/path"`
	APIKey string `json:"api_key" example:"my-api-key"`
}

// CreateLinkResponse is the response body returned when a link is successfully created.
type CreateLinkResponse struct {
	Slug     string `json:"slug"      example:"one-ring-to-rule"`
	ShortURL string `json:"short_url" example:"http://localhost:8080/one-ring-to-rule"`
	Original string `json:"original"  example:"https://example.com/some/very/long/path"`
}

// StatsResponse is the response body for link statistics.
type StatsResponse struct {
	Slug         string          `json:"slug"          example:"one-ring-to-rule"`
	Original     string          `json:"original"      example:"https://example.com"`
	ClickCount   int             `json:"click_count"   example:"42"`
	CreatedAt    time.Time       `json:"created_at"`
	ExpiresAt    time.Time       `json:"expires_at"`
	IsActive     bool            `json:"is_active"     example:"true"`
	RecentClicks []ClickResponse `json:"recent_clicks"`
}

// ClickResponse represents a single recorded click event.
type ClickResponse struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"       example:"one-ring-to-rule"`
	ClickedAt time.Time `json:"clicked_at"`
	Referrer  string    `json:"referrer"   example:"https://google.com"`
	UserAgent string    `json:"user_agent" example:"Mozilla/5.0"`
	IPAddress string    `json:"ip_address" example:"127.0.0.1"`
}

// ErrorResponse is a generic error envelope.
type ErrorResponse struct {
	Error string `json:"error" example:"slug not found"`
}
