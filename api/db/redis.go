package db

import (
	"context"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/wesley-lawson13/lembas-links/config"
)

func NewRedisClient(cfg *config.Config) *redis.Client {

	RedisUrl := strings.TrimPrefix(cfg.RedisURL, "redis://")
	client := redis.NewClient(&redis.Options{
		Addr: RedisUrl,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v\n", err)
	}

	log.Println("Successfully connected to Redis.")
	return client
}
