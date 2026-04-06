package models

import (
	"fmt"
)

func (s *URLStore) IncrementUseCount(slug string) error {

	query := `
        UPDATE quotes
        SET use_count = use_count + 1
        WHERE slug = $1
    `

	_, err := s.db.Exec(query, slug)
	if err != nil {
		return fmt.Errorf("failed to update use_count: %w", err)
	}

	return nil
}
