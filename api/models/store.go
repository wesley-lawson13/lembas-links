package models

import (
	"database/sql"
	"time"
)

type URLStore struct {
	db *sql.DB
}

type URL struct {
	ID         string
	Slug       string
	Original   string
	APIKey     string
	ClickCount int
	ExpiresAt  time.Time
	CreatedAt  time.Time
	IsActive   bool
}

type URLStats struct {
	Slug       string
	Original   string
	ClickCount int
	ExpiresAt  time.Time
	CreatedAt  time.Time
	IsActive   bool
}

func NewURLStore(db *sql.DB) *URLStore {
	return &URLStore{db: db}
}
