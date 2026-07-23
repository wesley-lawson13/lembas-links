package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/wesley-lawson13/lembas-links/config"
)

func RateLimit(r *redis.Client, cfg *config.Config) gin.HandlerFunc {

	window := time.Duration(cfg.RateLimitWindow) * time.Second

	return func(c *gin.Context) {

		ip := c.ClientIP()
		ipKey := fmt.Sprintf("rate:ip:%s", ip)

		ipCount, err := parseRate(c, r, ipKey, window)
		if err != nil {
			log.Printf("rate limiting failed on IP key %s: %v", ipKey, err)
			c.Next()
			return
		}

		if ipCount > cfg.IPRateLimit {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}

		authKey := c.GetHeader("Authorization")
		apiKey := fmt.Sprintf("rate:key:%s", ExtractBearerToken(authKey))

		if authKey == "" {
			c.Next()
			return
		}

		apiKeyCount, err := parseRate(c, r, apiKey, window)
		if err != nil {
			log.Printf("rate limiting failed on API key: %v", err)
			c.Next()
			return
		}

		if apiKeyCount > cfg.KeyRateLimit {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// SessionRateLimit is the sole rate limiter on POST /session (not stacked
// with RateLimit). It's IP-only and deliberately stricter than the general
// per-IP limit: minting a key is free and hands out a fresh per-key budget,
// so mint-rate is what actually bounds abuse now that keys are self-serve.
func SessionRateLimit(r *redis.Client, cfg *config.Config) gin.HandlerFunc {

	window := time.Duration(cfg.SessionRateLimitWindow) * time.Second

	return func(c *gin.Context) {

		ip := c.ClientIP()
		key := fmt.Sprintf("rate:session:%s", ip)

		count, err := parseRate(c, r, key, window)
		if err != nil {
			// Fail CLOSED here, unlike the general RateLimit which fails open.
			// Session minting is the abuse backstop: a fresh key hands out a new
			// per-key budget, so if Redis is down we'd rather refuse to mint than
			// let the cap be bypassed by simply knocking Redis over. The cost is
			// that new visitors can't onboard during a Redis outage; existing
			// keys still work (auth validates against Postgres, not Redis).
			log.Printf("session rate limiting failed on key %s: %v", key, err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable"})
			c.Abort()
			return
		}

		if count > cfg.SessionRateLimit {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func parseRate(c *gin.Context, r *redis.Client, key string, window time.Duration) (int, error) {

	// INCR is atomic and creates the key at 1 on first use, so counting can't
	// lose increments the way the old GET-then-INCR could under concurrent
	// bursts. We only need to stamp the fixed-window TTL on that first request.
	count, err := r.Incr(c, key).Result()
	if err != nil {
		return -1, fmt.Errorf("failed to increment rate: %w", err)
	}

	if count == 1 {
		// The tiny gap between INCR and EXPIRE would only matter if the process
		// died in between; if that's ever a concern, a Lua INCR+EXPIRE script is
		// the fully-atomic alternative.
		if err := r.Expire(c, key, window).Err(); err != nil {
			return -1, fmt.Errorf("failed to set rate window: %w", err)
		}
	}

	return int(count), nil
}
