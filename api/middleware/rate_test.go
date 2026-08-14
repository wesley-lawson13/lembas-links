package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/wesley-lawson13/lembas-links/config"
)

// setupTestRedis connects to the test Redis instance.
// Skips the test if TEST_REDIS_URL is not set.
func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	dsn := os.Getenv("TEST_REDIS_URL")
	if dsn == "" {
		t.Skip("TEST_REDIS_URL not set, skipping integration tests")
	}

	opts, err := redis.ParseURL(dsn)
	if err != nil {
		t.Fatalf("failed to parse TEST_REDIS_URL: %v", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Fatalf("failed to connect to test redis: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	return client
}

// newTestRouter wires the general-purpose RateLimit middleware (IP limit,
// plus a per-API-key limit when an Authorization header is present) in front
// of a dummy GET /. RateLimit fails open on Redis errors, since it guards
// normal traffic and shouldn't block legitimate requests during an outage.
func newTestRouter(cfg *config.Config, client *redis.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(client, cfg))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

// delAfterTest schedules Redis key deletion for after the test via
// t.Cleanup. It uses a fresh background context instead of t.Context():
// t.Context() is canceled just before Cleanup-registered functions run, so
// reusing it here makes the DEL fail silently and leaves the counter to
// expire on its own TTL, corrupting any second run inside that window.
func delAfterTest(t *testing.T, client *redis.Client, keys ...string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Del(ctx, keys...)
	})
}

func doRequest(r *gin.Engine, ip, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = fmt.Sprintf("%s:12345", ip)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// newSessionTestRouter wires the SessionRateLimit middleware (IP-only, since
// no API key exists yet) in front of a dummy POST /session. Unlike RateLimit,
// SessionRateLimit fails closed on Redis errors: /session is the abuse
// backstop for anonymous key minting, so an outage should refuse new keys
// rather than let it be bypassed by knocking Redis over.
func newSessionTestRouter(cfg *config.Config, client *redis.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SessionRateLimit(client, cfg))
	r.POST("/session", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})
	return r
}

func doSessionRequest(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/session", nil)
	req.RemoteAddr = fmt.Sprintf("%s:12345", ip)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestSessionRateLimit_FailsClosedWhenRedisUnavailable points the limiter at a
// dead Redis port so every command errors, and asserts the request is refused
// with 503 rather than admitted. This is the abuse backstop, so it must fail
// closed. Needs no TEST_REDIS_URL.
func TestSessionRateLimit_FailsClosedWhenRedisUnavailable(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { client.Close() })

	cfg := &config.Config{SessionRateLimit: 5, SessionRateLimitWindow: 3600}
	router := newSessionTestRouter(cfg, client)

	w := doSessionRequest(router, "10.10.80.1")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when redis is unavailable, got %d", w.Code)
	}
}

func TestSessionRateLimit_BlocksOverThreshold(t *testing.T) {
	client := setupTestRedis(t)
	ip := "10.10.81.1"
	key := fmt.Sprintf("rate:session:%s", ip)
	// Clear up front as well as on cleanup: a short window keeps a leftover
	// counter from an interrupted run from poisoning later runs.
	client.Del(t.Context(), key)
	delAfterTest(t, client, key)

	cfg := &config.Config{SessionRateLimit: 3, SessionRateLimitWindow: 60}
	router := newSessionTestRouter(cfg, client)

	for i := range 3 {
		if w := doSessionRequest(router, ip); w.Code != http.StatusCreated {
			t.Fatalf("request %d: expected 201 got %d", i+1, w.Code)
		}
	}

	if w := doSessionRequest(router, ip); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 over session threshold, got %d", w.Code)
	}
}

func TestRateLimit_AllowsRequestsUnderIPThreshold(t *testing.T) {
	client := setupTestRedis(t)
	ip := "10.10.10.1"
	delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ip))

	cfg := &config.Config{IPRateLimit: 5, KeyRateLimit: 100, RateLimitWindow: 60}
	router := newTestRouter(cfg, client)

	for i := range 5 {
		w := doRequest(router, ip, "")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 got %d", i+1, w.Code)
		}
	}
}

func TestRateLimit_BlocksRequestsOverIPThreshold(t *testing.T) {
	client := setupTestRedis(t)
	ip := "10.10.10.2"
	delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ip))

	cfg := &config.Config{IPRateLimit: 3, KeyRateLimit: 100, RateLimitWindow: 60}
	router := newTestRouter(cfg, client)

	for i := range 3 {
		w := doRequest(router, ip, "")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 got %d", i+1, w.Code)
		}
	}

	w := doRequest(router, ip, "")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 got %d", w.Code)
	}
}

// TestRateLimit_BlocksRequestsOverAPIKeyThreshold sets IPRateLimit far above
// what's sent and uses a different IP per request, so only the shared
// Authorization header can trip the limit — isolating the API-key counter
// from the IP counter and confirming it blocks once KeyRateLimit is exceeded.
func TestRateLimit_BlocksRequestsOverAPIKeyThreshold(t *testing.T) {
	client := setupTestRedis(t)
	authHeader := "Bearer test-rate-key-1"
	delAfterTest(t, client, fmt.Sprintf("rate:key:%s", ExtractBearerToken(authHeader)))

	cfg := &config.Config{IPRateLimit: 1000, KeyRateLimit: 3, RateLimitWindow: 60}
	router := newTestRouter(cfg, client)

	// use a different IP each request so the IP limit never interferes
	for i := range 3 {
		ip := fmt.Sprintf("10.10.20.%d", i+1)
		delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ip))

		w := doRequest(router, ip, authHeader)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 got %d", i+1, w.Code)
		}
	}

	ip := "10.10.20.99"
	delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ip))

	w := doRequest(router, ip, authHeader)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 got %d", w.Code)
	}
}

// TestRateLimit_PrefixedAndUnprefixedHeaderShareAPIKeyCounter alternates
// "Bearer <key>" and bare "<key>" across requests (different IP each time,
// so only the API-key counter can trip) and confirms they hit the same
// KeyRateLimit threshold together — proving ExtractBearerToken normalizes
// both forms to one counter instead of letting a caller double their budget
// by varying header format.
func TestRateLimit_PrefixedAndUnprefixedHeaderShareAPIKeyCounter(t *testing.T) {
	client := setupTestRedis(t)
	rawKey := "test-rate-key-2"
	delAfterTest(t, client, fmt.Sprintf("rate:key:%s", rawKey))

	cfg := &config.Config{IPRateLimit: 1000, KeyRateLimit: 3, RateLimitWindow: 60}
	router := newTestRouter(cfg, client)

	// alternate "Bearer <key>" and bare "<key>" across requests; if they
	// didn't share a counter, none of these would ever hit the threshold
	headers := []string{"Bearer " + rawKey, rawKey, "Bearer " + rawKey}
	for i, h := range headers {
		ip := fmt.Sprintf("10.10.60.%d", i+1)
		delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ip))

		w := doRequest(router, ip, h)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 got %d", i+1, w.Code)
		}
	}

	ip := "10.10.60.99"
	delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ip))

	w := doRequest(router, ip, rawKey)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 got %d (prefixed/unprefixed headers should share one counter)", w.Code)
	}
}

// TestRateLimit_NoAuthHeaderSkipsAPIKeyLimit sets KeyRateLimit well below the
// number of anonymous requests sent and confirms they all succeed, proving
// the API-key check is never applied when no Authorization header is present
// rather than just having a generous limit.
func TestRateLimit_NoAuthHeaderSkipsAPIKeyLimit(t *testing.T) {
	client := setupTestRedis(t)
	ip := "10.10.30.1"
	delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ip))

	// KeyRateLimit is set well below the number of requests made; if the
	// key-based check were mistakenly applied to anonymous requests, this
	// test would fail.
	cfg := &config.Config{IPRateLimit: 100, KeyRateLimit: 1, RateLimitWindow: 60}
	router := newTestRouter(cfg, client)

	for i := range 5 {
		w := doRequest(router, ip, "")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 got %d (key limit should not apply without Authorization header)", i+1, w.Code)
		}
	}
}

func TestRateLimit_DifferentIPsHaveIndependentCounters(t *testing.T) {
	client := setupTestRedis(t)
	ipA, ipB := "10.10.40.1", "10.10.40.2"
	delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ipA), fmt.Sprintf("rate:ip:%s", ipB))

	cfg := &config.Config{IPRateLimit: 2, KeyRateLimit: 1000, RateLimitWindow: 60}
	router := newTestRouter(cfg, client)

	for i := range 2 {
		if w := doRequest(router, ipA, ""); w.Code != http.StatusOK {
			t.Fatalf("ipA request %d: expected 200 got %d", i+1, w.Code)
		}
	}
	if w := doRequest(router, ipA, ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("ipA should be rate limited, got %d", w.Code)
	}

	// ipB should be unaffected by ipA's usage
	if w := doRequest(router, ipB, ""); w.Code != http.StatusOK {
		t.Fatalf("ipB should not be rate limited, got %d", w.Code)
	}
}

// TestRateLimit_IPWindowResetsAfterExpiry uses a 1-second RateLimitWindow to
// trip the IP counter, sleeps past it, and confirms a subsequent request from
// the same IP succeeds again — verifying the Redis counter actually expires
// on its TTL rather than blocking a caller permanently once tripped.
func TestRateLimit_IPWindowResetsAfterExpiry(t *testing.T) {
	client := setupTestRedis(t)
	ip := "10.10.50.1"
	delAfterTest(t, client, fmt.Sprintf("rate:ip:%s", ip))

	cfg := &config.Config{IPRateLimit: 1, KeyRateLimit: 1000, RateLimitWindow: 1}
	router := newTestRouter(cfg, client)

	if w := doRequest(router, ip, ""); w.Code != http.StatusOK {
		t.Fatalf("first request: expected 200 got %d", w.Code)
	}
	if w := doRequest(router, ip, ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429 got %d", w.Code)
	}

	time.Sleep(1200 * time.Millisecond)

	if w := doRequest(router, ip, ""); w.Code != http.StatusOK {
		t.Fatalf("after window expiry: expected 200 got %d", w.Code)
	}
}
