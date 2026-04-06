package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/db"
)

func main() {

	// load config
	cfg := config.Load()

	// connect to Postgres using connection pool
	pool := db.NewPool(cfg)
	defer pool.Close()

	// connect to Redis
	redis := db.NewRedisClient(cfg)
	defer redis.Close()

	// set up router
	r := gin.Default()

	// health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":   "ok",
			"service":  "lembas-links",
			"database": "connected",
			"cache":    "connected",
		})
	})

	// boot server
	addr := fmt.Sprintf(":%s", cfg.APIPort)
	log.Printf("Lembas Links api running on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %s", err)
	}
}
