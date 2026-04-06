package main

import (
	"fmt"
	"log"
    "database/sql"

	"github.com/gin-gonic/gin"
	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/db"
	"github.com/wesley-lawson13/lembas-links/models"

    // for migrations
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func runMigrations(pool *sql.DB) {
    driver, err := postgres.WithInstance(pool, &postgres.Config{})
    if err != nil {
        log.Fatalf("Failed to create migrate driver: %v", err)
    }

    m, err := migrate.NewWithDatabaseInstance(
        "file:///db/migrations",
        "postgres",
        driver,
    )
    if err != nil {
        log.Fatalf("Failed to create migrate instance: %v", err)
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        log.Fatalf("Failed to run migrations: %v", err)
    }

    log.Println("Migrations ran successfully")
}

func main() {

	// load config
	cfg := config.Load()

	// connect to Postgres using connection pool
	pool := db.NewPool(cfg)
	defer pool.Close()

    // run migrations
    runMigrations(pool)

	// connect to Redis
	redis := db.NewRedisClient(cfg)
	defer redis.Close()

    // set up the store for the db
    store := models.NewURLStore(pool)

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

    // empty statement s.t. Go will not throw an error here
    _ = store
}
