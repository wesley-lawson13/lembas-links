package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	APIPort     string
	BaseURL     string

	// middleware vars
	IPRateLimit            int
	KeyRateLimit           int
	RateLimitWindow        int // in seconds
	DefaultTTLDays         int
	SessionRateLimit       int
	SessionRateLimitWindow int // in seconds

	// CORS
	CORSAllowedOrigins []string

	// analytics
	RecentClicksLimit int
}

func Load() *Config {

	cfg := Config{
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		RedisURL:               os.Getenv("REDIS_URL"),
		APIPort:                getEnvWithFallback("API_PORT", "PORT", "8080"),
		BaseURL:                os.Getenv("BASE_URL"),
		IPRateLimit:            getEnvInt("IP_RATE_LIMIT", 60),
		KeyRateLimit:           getEnvInt("KEY_RATE_LIMIT", 120),
		RateLimitWindow:        getEnvInt("RATE_LIMIT_WINDOW", 60),
		DefaultTTLDays:         getEnvInt("DEFAULT_TTL_DAYS", 30),
		SessionRateLimit:       getEnvInt("SESSION_RATE_LIMIT", 5),
		SessionRateLimitWindow: getEnvInt("SESSION_RATE_LIMIT_WINDOW", 3600),
		CORSAllowedOrigins:     getEnvList("CORS_ALLOWED_ORIGINS"),
		RecentClicksLimit:      getEnvInt("RECENT_CLICKS_LIMIT", 10),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	if cfg.RedisURL == "" {
		log.Fatal("REDIS_URL is required")
	}

	return &cfg
}

func getEnvWithFallback(keys ...string) string {
	// All but the last element are env var keys; last element is the default value.
	for _, key := range keys[:len(keys)-1] {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return keys[len(keys)-1]
}

func getEnvList(key string) []string {

	val := os.Getenv(key)
	if val == "" {
		return nil
	}

	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

func getEnvInt(key string, defaultVal int) int {

	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("failed to parse %s=%q as integer, using default %d", key, val, defaultVal)
		return defaultVal
	}

	return parsed
}
