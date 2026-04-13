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
		apiKey := fmt.Sprintf("rate:key:%s", authKey)

		if apiKey == "" {
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

func parseRate(c *gin.Context, r *redis.Client, key string, window time.Duration) (int, error) {

	_, err := r.Get(c, key).Int()
	if err == redis.Nil {
		r.Set(c, key, 1, window)
		return 1, nil
	} else if err != nil {
		return -1, fmt.Errorf("failed to get rate: %w", err)
	}

	newCount, err := r.Incr(c, key).Result()
	if err != nil {
		return -1, fmt.Errorf("failed to increment rate: %w", err)
	}

	return int(newCount), nil
}
