// @title           Lembas Links API
// @version         1.0
// @description     A Lord of the Rings-themed URL shortener. Authenticated routes require an API key passed as a Bearer token in the Authorization header.
// @host            localhost:8080
// @BasePath        /
//
// @securityDefinitions.apikey ApiKeyAuth
// @in                         header
// @name                       Authorization
// @description                API key prefixed with "Bearer " (e.g. "Bearer my-api-key")
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// generated swagger docs (run `swag init` to create/update)
	_ "github.com/wesley-lawson13/lembas-links/docs"

	// local files
	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/db"
	"github.com/wesley-lawson13/lembas-links/handlers"
	"github.com/wesley-lawson13/lembas-links/middleware"
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

func seedQuotesIfEmpty(pool *sql.DB) {
	var count int
	if err := pool.QueryRow("SELECT COUNT(*) FROM quotes").Scan(&count); err != nil {
		log.Printf("Seed check failed: %v", err)
		return
	}
	if count > 0 {
		log.Println("Quotes table already seeded, skipping")
		return
	}

	sqlBytes, err := os.ReadFile("/db/seeds/quotes.sql")
	if err != nil {
		log.Printf("Failed to read quotes seed file: %v", err)
		return
	}
	if _, err := pool.Exec(string(sqlBytes)); err != nil {
		log.Printf("Failed to seed quotes: %v", err)
		return
	}
	log.Println("Quotes table seeded successfully")
}

func main() {

	// load config
	cfg := config.Load()

	// connect to Postgres using connection pool
	pool := db.NewPool(cfg)
	defer pool.Close()

	// run migrations
	runMigrations(pool)

	// seed quotes table if empty (first deploy)
	seedQuotesIfEmpty(pool)

	// connect to Redis
	redis := db.NewRedisClient(cfg)
	defer redis.Close()

	// set up the store for the db
	store := models.NewURLStore(pool)

	// set up router
	r := gin.Default()

	// get the link handler for routes
	linkHandler := handlers.NewLinkHandler(store, redis, cfg)

	// ---ROUTES---

	// public routes
	r.GET("/health", func(c *gin.Context) {
		// health check
		c.JSON(200, gin.H{
			"status":   "ok",
			"service":  "lembas-links",
			"database": "connected",
			"cache":    "connected",
		})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/:slug", linkHandler.Redirect)

	// protected routes
	protected := r.Group("/links")
	protected.Use(middleware.APIKeyAuth(store))
	{
		protected.POST("", linkHandler.CreateLink)
		protected.DELETE("/:slug", linkHandler.DeleteLink)
		protected.GET("/:slug/stats", linkHandler.GetStats)
	}

	addr := fmt.Sprintf(":%s", cfg.APIPort)
	log.Printf("Lembas Links api running on %s", addr)

	// boot server - blocks while server is running
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %s", err)
	}
}
