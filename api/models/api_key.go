package models

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// hashKey returns the SHA-256 hex digest of a raw API key. SHA-256 (rather
// than a slow password hash) is appropriate here since keys are high-entropy
// random tokens, not user-chosen passwords.
func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// HashKey exposes hashKey for other packages (e.g. handlers) that need to
// hash a caller-supplied key to compare it against stored hashes.
func HashKey(rawKey string) string {
	return hashKey(rawKey)
}

// generateRawKey returns a random hex-encoded key using n bytes of entropy
// from crypto/rand.
func generateRawKey(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate api key: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// CreateKey generates a new random API key, stores its hash with an
// expires_at of now+ttl, and returns the raw key. The raw key is never
// persisted and cannot be recovered after this call returns.
func (s *URLStore) CreateKey(ttl time.Duration) (string, error) {
	rawKey, err := generateRawKey(32)
	if err != nil {
		return "", err
	}

	query := `
        INSERT INTO api_keys (key, expires_at)
        VALUES ($1, $2)
    `

	// api_keys.expires_at is TIMESTAMP (no time zone): Postgres stores
	// whatever wall-clock digits it's given, discarding any offset. Convert
	// to UTC first so those digits line up with the DB's NOW(), which runs
	// in the UTC session time zone.
	_, err = s.db.Exec(query, hashKey(rawKey), time.Now().Add(ttl).UTC())
	if err != nil {
		return "", fmt.Errorf("failed to create api key: %w", err)
	}

	return rawKey, nil
}

// ValidateKey checks that key is a valid, unexpired API key and, if so,
// slides its expires_at forward by ttl in the same round trip (avoiding a
// separate check-then-extend race).
func (s *URLStore) ValidateKey(key string, ttl time.Duration) error {

	query := `
        UPDATE api_keys
        SET expires_at = NOW() + (INTERVAL '1 second' * $2)
        WHERE key = $1 AND expires_at > NOW()
        RETURNING id
    `
	var id string

	err := s.db.QueryRow(query, hashKey(key), ttl.Seconds()).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("api key not found or expired")
	}
	if err != nil {
		return fmt.Errorf("failed to validate api key: %w", err)
	}

	return nil
}
