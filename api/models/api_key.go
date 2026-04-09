package models

import (
	"database/sql"
	"fmt"
)

func (s *URLStore) ValidateKey(key string) error {

	query := `
        SELECT key
        FROM api_keys
        WHERE key = $1
    `
	var foundKey string

	err := s.db.QueryRow(query, key).Scan(&foundKey)
	if err == sql.ErrNoRows {
		return fmt.Errorf("api key not found")
	}
	if err != nil {
		return fmt.Errorf("failed to validate api key: %w", err)
	}

	return nil
}
