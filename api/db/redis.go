package db

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wesley-lawson13/lembas-links/config"
)

func NewRedisClient(cfg *config.Config) *redis.Client {

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v\n", err)
	}
	client := redis.NewClient(opts)

	// connection retry ping — on separate hosts the app box can boot before
	// the data box is reachable, so a single ping would crash-loop it.
	for i := range 10 {

		if err := client.Ping(context.Background()).Err(); err == nil {
			log.Println("Successfully connected to Redis.")
			return client
		} else {
			log.Printf("Attempt %d/10 failed: %v\n", i+1, err)
		}
		time.Sleep(2 * time.Second)
	}

	log.Fatal("Could not connect to Redis.")
	return nil
}
