package models

import (
	"database/sql"
	_ "github.com/lib/pq"
	"testing"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("postgres", "postgres://subtub13:BellaMookie_dev_pw@localhost:5432/lembas_links?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	return db
}

func TestGetSlug(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	slug, err := store.GetSlug()
	if err != nil {
		t.Fatalf("GetSlug failed: %v", err)
	}
	if slug == "" {
		t.Fatal("Expected a slug but got empty string")
	}
	t.Logf("Got slug: %s", slug)
}

func TestCreateAndGetURL(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	// First get a slug
	slug, err := store.GetSlug()
	if err != nil {
		t.Fatalf("GetSlug failed: %v", err)
	}

	// Create a URL with it
	err = store.CreateURL(slug, "https://example.com", "test-api-key")
	if err != nil {
		t.Fatalf("CreateURL failed: %v", err)
	}

	// Get it back
	url, err := store.GetURL(slug)
	if err != nil {
		t.Fatalf("GetURL failed: %v", err)
	}
	if url.Original != "https://example.com" {
		t.Fatalf("Expected https://example.com but got %s", url.Original)
	}
	t.Logf("Created and retrieved URL: %s -> %s", url.Slug, url.Original)
}

func TestDeleteURL(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	slug, _ := store.GetSlug()
	store.CreateURL(slug, "https://delete-test.com", "test-key")

	err := store.DeleteURL(slug)
	if err != nil {
		t.Fatalf("DeleteURL failed: %v", err)
	}

	// Verify is_active is false
	url, _ := store.GetURL(slug)
	if url.IsActive {
		t.Fatal("Expected is_active to be false after deletion")
	}
	t.Log("URL successfully soft deleted")
}
