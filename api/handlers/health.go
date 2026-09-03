package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	pool  *sql.DB
	redis *redis.Client
}

func NewHealthHandler(pool *sql.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{pool: pool, redis: redis}
}

// Check godoc
// @Summary      Readiness check
// @Description  Pings Postgres and Redis in parallel (2s timeout) and reports each dependency's live status.
// @Tags         health
// @Produce      json
// @Success      200 {object} HealthResponse
// @Failure      503 {object} HealthResponse "one or more dependencies unreachable"
// @Router       /health [get]
func (hh *HealthHandler) Check(c *gin.Context) {

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	var dbErr, cacheErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); dbErr = hh.pool.PingContext(ctx) }()
	go func() { defer wg.Done(); cacheErr = hh.redis.Ping(ctx).Err() }()
	wg.Wait()

	healthy := dbErr == nil && cacheErr == nil
	status, httpStatus := "ok", http.StatusOK
	if !healthy {
		status, httpStatus = "degraded", http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":   status,
		"service":  "lembas-links",
		"database": connectionStatus(dbErr),
		"cache":    connectionStatus(cacheErr),
	})
}

// connectionStatus renders a dependency's ping error as the /health response
// string. Called by Check for both the Postgres pool and Redis.
func connectionStatus(err error) string {
	if err != nil {
		return "disconnected"
	}
	return "connected"
}
