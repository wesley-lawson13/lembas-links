// @title           Lembas Links API
// @version         1.0
// @description     A Lord of the Rings-themed URL shortener. Authenticated routes require an API key passed as a Bearer token in the Authorization header.
// @host            lembas-links-production.up.railway.app
// @BasePath        /
//
// @securityDefinitions.apikey ApiKeyAuth
// @in                         header
// @name                       Authorization
// @description                API key prefixed with "Bearer " (e.g. "Bearer my-api-key")
package main

import (
	"fmt"
	"log"

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
	"github.com/wesley-lawson13/lembas-links/migrate"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {

	// load config
	cfg := config.Load()

	// connect to Postgres using connection pool
	pool := db.NewPool(cfg)
	defer pool.Close()

	// run migrations
	migrate.RunMigrations(pool)

	// seed quotes table if empty (first deploy)
	migrate.SeedQuotesIfEmpty(pool)

	// connect to Redis
	redis := db.NewRedisClient(cfg)
	defer redis.Close()

	// set up the store for the db
	store := models.NewURLStore(pool)

	// set up router
	r := gin.Default()

	// get the link and session handlers for routes
	linkHandler := handlers.NewLinkHandler(store, redis, cfg)
	sessionHandler := handlers.NewSessionHandler(store, cfg)

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

	r.POST("/session", middleware.SessionRateLimit(redis, cfg), sessionHandler.CreateSession)

	r.GET("/:slug", middleware.RateLimit(redis, cfg), linkHandler.Redirect)

	// protected routes
	protected := r.Group("/links")
	protected.Use(middleware.RateLimit(redis, cfg), middleware.APIKeyAuth(store, cfg))
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
