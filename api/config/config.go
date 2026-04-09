package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	APIPort     string
	APISecret   string
	BaseURL     string

	// middleware vars
	IPRateLimit    int
	KeyRateLimit   int
	DefaultTTLDays int
}

func Load() *Config {

	cfg := Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		RedisURL:       os.Getenv("REDIS_URL"),
		APIPort:        os.Getenv("API_PORT"),
		APISecret:      os.Getenv("API_SECRET_KEY"),
		BaseURL:        os.Getenv("BASE_URL"),
		IPRateLimit:    getEnvInt("API_RATE_LIMIT", 60),
		KeyRateLimit:   getEnvInt("KEY_RATE_LIMIT", 120),
		DefaultTTLDays: getEnvInt("DEFAULT_TTL_DAYS", 30),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	if cfg.RedisURL == "" {
		log.Fatal("REDIS_URL is required")
	}

	return &cfg
}

func getEnvInt(key string, defaultVal int) int {

	val := os.Getenv(key)
	if val == "" {
		log.Println("failed to access environment integer val")
		return defaultVal
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Println("failed to parse environment integer val")
		return defaultVal
	}

	return parsed
}
