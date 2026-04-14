package db

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/wesley-lawson13/lembas-links/config"
)

func NewRedisClient(cfg *config.Config) *redis.Client {

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v\n", err)
	}
	client := redis.NewClient(opts)

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v\n", err)
	}

	log.Println("Successfully connected to Redis.")
	return client
}
