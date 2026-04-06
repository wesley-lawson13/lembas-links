package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	APIPort     string
	APISecret   string
}

func Load() *Config {

	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		APIPort:     os.Getenv("API_PORT"),
		APISecret:   os.Getenv("API_SECRET_KEY"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	if cfg.RedisURL == "" {
		log.Fatal("REDIS_URL is required")
	}

	return &cfg
}
