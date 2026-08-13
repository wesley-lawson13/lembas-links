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

// seedURL inserts a url row for the given api_key under a least-used quote slug,
// standing in for the removed store.GetSlug + store.CreateURL pair. Called from
// the model tests that need a pre-existing link to act on. Unlike the old
// GetSlug, it skips slugs already taken in urls: nothing here bumps use_count,
// so two seeds in one test would otherwise draw the same slug and collide.
func seedURL(t *testing.T, db *sql.DB, original, apiKey string, expiresAt time.Time) string {
	t.Helper()

	var slug string
	err := db.QueryRow(`
        SELECT q.slug
        FROM quotes q
        WHERE NOT EXISTS (SELECT 1 FROM urls u WHERE u.slug = q.slug)
        ORDER BY q.use_count, RANDOM()
        LIMIT 1
    `).Scan(&slug)
	if err != nil {
		t.Fatalf("failed to pick a test slug: %v", err)
	}

	_, err = db.Exec(`
        INSERT INTO urls (slug, original, api_key, expires_at)
        VALUES ($1, $2, $3, $4)
    `, slug, original, apiKey, expiresAt)
	if err != nil {
		t.Fatalf("failed to seed test url %q: %v", slug, err)
	}

	return slug
}

func TestURLLifecycle(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)
	cleanupURLs(t, db, "test-lifecycle-key")

	// Step 1 — create a URL
	slug := seedURL(t, db, "https://example.com", "test-lifecycle-key", time.Now().Add(30*24*time.Hour))

	// Step 2 — retrieve it
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

	// Step 3 — increment click count
	err = store.IncrementClickCount(slug)
	if err != nil {
		t.Fatalf("IncrementClickCount failed: %v", err)
	}

	// Step 4 — verify stats
	stats, err := store.GetStats(slug)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.ClickCount != 1 {
		t.Errorf("expected click_count 1 got %d", stats.ClickCount)
	}

	// Step 5 — soft delete
	err = store.DeleteURL(slug)
	if err != nil {
		t.Fatalf("DeleteURL failed: %v", err)
	}

	// Step 6 — verify inactive
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
	slug := seedURL(t, db, "https://example.com", "test-clicks-key", time.Now().Add(30*24*time.Hour))

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

// TestGetDailyClicks verifies GetDailyClicks returns exactly 7 zero-filled
// per-day totals, oldest to newest, counting only clicks inside the window.
func TestGetDailyClicks(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)
	cleanupURLs(t, db, "test-daily-clicks-key")

	t.Run("backdated clicks land in the right buckets", func(t *testing.T) {
		slug := seedURL(t, db, "https://example.com", "test-daily-clicks-key", time.Now().Add(30*24*time.Hour))

		// RecordClick relies on the clicked_at NOW() default so it can't
		// backdate — insert clicks with explicit timestamps directly.
		inserts := []struct {
			interval string
			count    int
		}{
			{"0 days", 2}, // today
			{"3 days", 3}, // inside the 7-day window
			{"8 days", 1}, // outside the window, must not be counted
		}
		for _, ins := range inserts {
			for i := 0; i < ins.count; i++ {
				_, err := db.Exec(
					"INSERT INTO clicks (slug, clicked_at, referrer, user_agent, ip_address) VALUES ($1, NOW() - $2::interval, '', '', '')",
					slug, ins.interval,
				)
				if err != nil {
					t.Fatalf("failed to insert backdated click (%s): %v", ins.interval, err)
				}
			}
		}

		daily, err := store.GetDailyClicks(slug, 7)
		if err != nil {
			t.Fatalf("GetDailyClicks failed: %v", err)
		}
		if len(daily) != 7 {
			t.Fatalf("expected 7 entries, got %d", len(daily))
		}

		// dates strictly ascending, last entry is today's UTC date
		for i := 1; i < len(daily); i++ {
			if daily[i].Date <= daily[i-1].Date {
				t.Errorf("expected strictly ascending dates, got %s then %s", daily[i-1].Date, daily[i].Date)
			}
		}
		today := time.Now().UTC().Format("2006-01-02")
		if daily[6].Date != today {
			t.Errorf("expected last entry to be today %s, got %s", today, daily[6].Date)
		}

		// index 6 = today (2 clicks), index 3 = 3 days ago (3 clicks), all
		// others zero; total 5 proves the 8-day-old click is nowhere counted
		total := 0
		for i, d := range daily {
			total += d.Count
			want := 0
			switch i {
			case 3:
				want = 3
			case 6:
				want = 2
			}
			if d.Count != want {
				t.Errorf("entry %d (%s): expected count %d, got %d", i, d.Date, want, d.Count)
			}
		}
		if total != 5 {
			t.Errorf("expected 5 clicks in window (8-day-old click excluded), got %d", total)
		}
	})

	t.Run("slug with zero clicks returns 7 zero entries", func(t *testing.T) {
		slug := seedURL(t, db, "https://example.com", "test-daily-clicks-key", time.Now().Add(30*24*time.Hour))

		daily, err := store.GetDailyClicks(slug, 7)
		if err != nil {
			t.Fatalf("GetDailyClicks failed: %v", err)
		}
		if len(daily) != 7 {
			t.Fatalf("expected 7 entries for clickless slug, got %d", len(daily))
		}
		for i, d := range daily {
			if d.Count != 0 {
				t.Errorf("entry %d (%s): expected count 0, got %d", i, d.Date, d.Count)
			}
		}
	})
}

func TestListURLsByAPIKey(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	keyA := "test-list-key-a"
	keyB := "test-list-key-b"
	cleanupURLs(t, db, hashKey(keyA))
	cleanupURLs(t, db, hashKey(keyB))

	slugA1 := seedURL(t, db, "https://example.com/a1", hashKey(keyA), time.Now().Add(30*24*time.Hour))
	slugA2 := seedURL(t, db, "https://example.com/a2", hashKey(keyA), time.Now().Add(30*24*time.Hour))
	// key B's link exists only to prove it never shows up in key A's list
	seedURL(t, db, "https://example.com/b1", hashKey(keyB), time.Now().Add(30*24*time.Hour))

	urls, err := store.ListURLsByAPIKey(keyA)
	if err != nil {
		t.Fatalf("ListURLsByAPIKey failed: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls for key A, got %d", len(urls))
	}

	if err := store.DeleteURL(slugA1); err != nil {
		t.Fatalf("DeleteURL failed: %v", err)
	}

	urls, err = store.ListURLsByAPIKey(keyA)
	if err != nil {
		t.Fatalf("ListURLsByAPIKey failed: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected 1 url for key A after soft-delete, got %d", len(urls))
	}
	if urls[0].Slug != slugA2 {
		t.Errorf("expected remaining url to be %s, got %s", slugA2, urls[0].Slug)
	}
}

// cleanupAPIKey removes a test API key (by its hash) after the test.
func cleanupAPIKey(t *testing.T, db *sql.DB, hashedKey string) {
	t.Helper()
	t.Cleanup(func() {
		db.Exec("DELETE FROM api_keys WHERE key = $1", hashedKey)
	})
}

func TestValidateKey(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	rawKey, err := store.CreateKey(24 * time.Hour)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}
	cleanupAPIKey(t, db, hashKey(rawKey))

	cases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid key", rawKey, false},
		{"invalid key", "not-a-real-key", true},
		{"empty key", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := store.ValidateKey(tc.key, 24*time.Hour)
			if tc.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestCreateAndValidateKeyRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	rawKey, err := store.CreateKey(24 * time.Hour)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}
	cleanupAPIKey(t, db, hashKey(rawKey))

	if err := store.ValidateKey(rawKey, 24*time.Hour); err != nil {
		t.Errorf("expected freshly created key to validate, got: %v", err)
	}
}

func TestValidateKeyExpired(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	rawKey, err := generateRawKey(32)
	if err != nil {
		t.Fatalf("generateRawKey failed: %v", err)
	}
	hashedKey := hashKey(rawKey)
	cleanupAPIKey(t, db, hashedKey)

	_, err = db.Exec(
		"INSERT INTO api_keys (key, expires_at) VALUES ($1, $2)",
		hashedKey, time.Now().Add(-1*time.Hour),
	)
	if err != nil {
		t.Fatalf("failed to insert expired key: %v", err)
	}

	if err := store.ValidateKey(rawKey, 24*time.Hour); err == nil {
		t.Error("expected expired key to fail validation")
	}
}

func TestValidateKeySlidesExpiry(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	rawKey, err := store.CreateKey(1 * time.Hour)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}
	hashedKey := hashKey(rawKey)
	cleanupAPIKey(t, db, hashedKey)

	var before time.Time
	if err := db.QueryRow("SELECT expires_at FROM api_keys WHERE key = $1", hashedKey).Scan(&before); err != nil {
		t.Fatalf("failed to read initial expires_at: %v", err)
	}

	if err := store.ValidateKey(rawKey, 24*time.Hour); err != nil {
		t.Fatalf("ValidateKey failed: %v", err)
	}

	var after time.Time
	if err := db.QueryRow("SELECT expires_at FROM api_keys WHERE key = $1", hashedKey).Scan(&after); err != nil {
		t.Fatalf("failed to read slid expires_at: %v", err)
	}

	if !after.After(before) {
		t.Errorf("expected expires_at to slide forward, before=%v after=%v", before, after)
	}
}

func TestSlugForUseCount(t *testing.T) {
	tests := []struct {
		base     string
		useCount int
		want     string
	}{
		{"one-ring-to-rule", 1, "one-ring-to-rule"},
		{"one-ring-to-rule", 2, "one-ring-to-rule-2"},
		{"one-ring-to-rule", 3, "one-ring-to-rule-3"},
		{"shadow", 0, "shadow"}, // defensive: non-positive treated as first use
	}
	for _, tc := range tests {
		if got := slugForUseCount(tc.base, tc.useCount); got != tc.want {
			t.Errorf("slugForUseCount(%q, %d) = %q, want %q", tc.base, tc.useCount, got, tc.want)
		}
	}
}

// TestAllocateURL forces one dedicated quote to be the strict minimum (all
// others bumped high) so it's reused on every allocation, proving the reuse
// counter produces base, base-2, base-3 with no collision and no spurious error
// on the 2nd/3rd call — the exact case the old GetSlug+CreateURL two-step 500'd.
func TestAllocateURL(t *testing.T) {
	db := setupTestDB(t)
	store := NewURLStore(db)

	const (
		testBase = "alloc-test-base-slug"
		apiKey   = "alloc-test-key"
	)

	if _, err := db.Exec("UPDATE quotes SET use_count = 999999"); err != nil {
		t.Fatalf("failed to bump quotes: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO quotes (quote, slug, use_count) VALUES ($1, $2, 0)",
		"test quote", testBase,
	); err != nil {
		t.Fatalf("failed to insert test quote: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM urls WHERE api_key = $1", apiKey)
		db.Exec("DELETE FROM quotes WHERE slug = $1", testBase)
		db.Exec("UPDATE quotes SET use_count = 0")
	})

	exp := time.Now().Add(24 * time.Hour)
	var got []string
	for i := range 3 {
		slug, err := store.AllocateURL("https://example.com", apiKey, exp)
		if err != nil {
			t.Fatalf("AllocateURL attempt %d failed: %v", i+1, err)
		}
		got = append(got, slug)
	}

	want := []string{testBase, testBase + "-2", testBase + "-3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("allocation %d: got slug %q, want %q", i+1, got[i], want[i])
		}
	}

	// every allocated slug is distinct and persisted exactly once
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s] {
			t.Fatalf("duplicate slug allocated: %q", s)
		}
		seen[s] = true

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM urls WHERE slug = $1", s).Scan(&count); err != nil {
			t.Fatalf("query url %q: %v", s, err)
		}
		if count != 1 {
			t.Fatalf("expected 1 url row for slug %q, got %d", s, count)
		}
	}
}
