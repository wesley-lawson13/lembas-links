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
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
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
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/wesley-lawson13/lembas-links/migrate"
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

	// nginx is the only proxy in front of the API (both on the same host in
	// prod), so trusting anything broader lets a client spoof X-Forwarded-For
	// and defeat the per-IP rate limit and click analytics.
	if err := r.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		log.Fatalf("Failed to set trusted proxies: %v", err)
	}

	// CORS must run before any route group so preflight OPTIONS requests
	// never hit RateLimit/APIKeyAuth/SessionRateLimit.
	// gin-contrib/cors panics on an empty AllowOrigins list, so skip
	// registering it entirely when unconfigured rather than crash the API;
	// non-browser callers (curl, health checks) are unaffected either way
	// since CORS only ever applies to requests carrying an Origin header.
	if len(cfg.CORSAllowedOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.CORSAllowedOrigins,
			AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Authorization", "Content-Type"},
			AllowCredentials: false,
			MaxAge:           12 * time.Hour,
		}))
	}

	// get the link and session handlers for routes
	linkHandler := handlers.NewLinkHandler(store, redis, cfg)
	sessionHandler := handlers.NewSessionHandler(store, cfg)

	// ---ROUTES---

	// public routes
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		var dbErr, cacheErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); dbErr = pool.PingContext(ctx) }()
		go func() { defer wg.Done(); cacheErr = redis.Ping(ctx).Err() }()
		wg.Wait()

		healthy := dbErr == nil && cacheErr == nil
		status, httpStatus := "ok", 200
		if !healthy {
			status, httpStatus = "degraded", 503
		}

		c.JSON(httpStatus, gin.H{
			"status":   status,
			"service":  "lembas-links",
			"database": connectionStatus(dbErr),
			"cache":    connectionStatus(cacheErr),
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
		protected.GET("", linkHandler.ListLinks)
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

// connectionStatus renders a dependency's ping error as the /health response
// string. Called by the /health handler for both the Postgres pool and Redis.
func connectionStatus(err error) string {
	if err != nil {
		return "disconnected"
	}
	return "connected"
}
