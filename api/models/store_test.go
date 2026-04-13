package models

import (
    "database/sql"
    "os"
    "testing"
    "time"

    _ "github.com/lib/pq"
)

// setupTestDB connects to the test database.
// Skips the test if TEST_DATABASE_URL is not set.
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
    }
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        t.Fatalf("failed to connect to test database: %v", err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}

// cleanupURLs removes test URLs after each test
func cleanupURLs(t *testing.T, db *sql.DB, apiKey string) {
    t.Helper()
    t.Cleanup(func() {
        db.Exec("DELETE FROM clicks WHERE slug IN (SELECT slug FROM urls WHERE api_key = $1)", apiKey)
        db.Exec("DELETE FROM urls WHERE api_key = $1", apiKey)
    })
}

func TestGetSlug(t *testing.T) {
    db := setupTestDB(t)
    store := NewURLStore(db)

    slug, err := store.GetSlug()
    if err != nil {
        t.Fatalf("GetSlug failed: %v", err)
    }
    if slug == "" {
        t.Fatal("expected a slug but got empty string")
    }
    t.Logf("got slug: %s", slug)
}

func TestGetSlugNotFound(t *testing.T) {
    db := setupTestDB(t)
    store := NewURLStore(db)

    // Temporarily empty the quotes table
    db.Exec("UPDATE quotes SET use_count = 999999")
    t.Cleanup(func() {
        db.Exec("UPDATE quotes SET use_count = 0")
    })

    // Should still return a slug since use_count just offsets
    _, err := store.GetSlug()
    if err != nil {
        t.Fatalf("GetSlug should always return a slug: %v", err)
    }
}

func TestURLLifecycle(t *testing.T) {
    db := setupTestDB(t)
    store := NewURLStore(db)
    cleanupURLs(t, db, "test-lifecycle-key")

    // Step 1 — get a slug
    slug, err := store.GetSlug()
    if err != nil {
        t.Fatalf("GetSlug failed: %v", err)
    }

    // Step 2 — create a URL
    err = store.CreateURL(slug, "https://example.com", "test-lifecycle-key", time.Now().Add(30*24*time.Hour))
    if err != nil {
        t.Fatalf("CreateURL failed: %v", err)
    }

    // Step 3 — retrieve it
    url, err := store.GetURL(slug)
    if err != nil {
        t.Fatalf("GetURL failed: %v", err)
    }
    if url.Original != "https://example.com" {
        t.Errorf("expected https://example.com got %s", url.Original)
    }
    if !url.IsActive {
        t.Error("expected url to be active")
    }

    // Step 4 — increment click count
    err = store.IncrementClickCount(slug)
    if err != nil {
        t.Fatalf("IncrementClickCount failed: %v", err)
    }

    // Step 5 — verify stats
    stats, err := store.GetStats(slug)
    if err != nil {
        t.Fatalf("GetStats failed: %v", err)
    }
    if stats.ClickCount != 1 {
        t.Errorf("expected click_count 1 got %d", stats.ClickCount)
    }

    // Step 6 — soft delete
    err = store.DeleteURL(slug)
    if err != nil {
        t.Fatalf("DeleteURL failed: %v", err)
    }

    // Step 7 — verify inactive
    url, err = store.GetURL(slug)
    if err != nil {
        t.Fatalf("GetURL after delete failed: %v", err)
    }
    if url.IsActive {
        t.Error("expected url to be inactive after delete")
    }
}

func TestGetURLNotFound(t *testing.T) {
    db := setupTestDB(t)
    store := NewURLStore(db)

    _, err := store.GetURL("this-slug-does-not-exist")
    if err == nil {
        t.Fatal("expected error for nonexistent slug but got nil")
    }
}

func TestRecordAndGetClicks(t *testing.T) {
    db := setupTestDB(t)
    store := NewURLStore(db)
    cleanupURLs(t, db, "test-clicks-key")

    // Create a URL first
    slug, _ := store.GetSlug()
    store.CreateURL(slug, "https://example.com", "test-clicks-key", time.Now().Add(30*24*time.Hour))

    // Record some clicks
    for i := 0; i < 3; i++ {
        err := store.RecordClick(slug, "https://google.com", "curl/8.4.0", "192.168.1.1")
        if err != nil {
            t.Fatalf("RecordClick failed: %v", err)
        }
    }

    // Retrieve clicks
    clicks, err := store.GetClicks(slug, 10)
    if err != nil {
        t.Fatalf("GetClicks failed: %v", err)
    }
    if len(clicks) != 3 {
        t.Errorf("expected 3 clicks got %d", len(clicks))
    }

    // Verify order — most recent first
    if clicks[0].ClickedAt.Before(clicks[1].ClickedAt) {
        t.Error("expected clicks ordered most recent first")
    }
}

func TestValidateKey(t *testing.T) {
    db := setupTestDB(t)
    store := NewURLStore(db)

    cases := []struct {
        name    string
        key     string
        wantErr bool
    }{
        {"valid key", "test-api-key-123", false},
        {"invalid key", "not-a-real-key", true},
        {"empty key", "", true},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            err := store.ValidateKey(tc.key)
            if tc.wantErr && err == nil {
                t.Error("expected error but got nil")
            }
            if !tc.wantErr && err != nil {
                t.Errorf("expected no error but got: %v", err)
            }
        })
    }
}

func TestIncrementUseCount(t *testing.T) {
    db := setupTestDB(t)
    store := NewURLStore(db)

    slug, err := store.GetSlug()
    if err != nil {
        t.Fatalf("GetSlug failed: %v", err)
    }

    // Get current use count
    var before int
    db.QueryRow("SELECT use_count FROM quotes WHERE slug = $1", slug).Scan(&before)

    err = store.IncrementUseCount(slug)
    if err != nil {
        t.Fatalf("IncrementUseCount failed: %v", err)
    }

    var after int
    db.QueryRow("SELECT use_count FROM quotes WHERE slug = $1", slug).Scan(&after)

    if after != before+1 {
        t.Errorf("expected use_count %d got %d", before+1, after)
    }

    // Cleanup
    t.Cleanup(func() {
        db.Exec("UPDATE quotes SET use_count = $1 WHERE slug = $2", before, slug)
    })
}
