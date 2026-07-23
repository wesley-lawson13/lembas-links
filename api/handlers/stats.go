package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wesley-lawson13/lembas-links/middleware"
	"github.com/wesley-lawson13/lembas-links/models"
)

// dailyClickDays is fixed at 7 because the chart on the frontend is fixed at 7 columns.
const dailyClickDays = 7

// GetStats godoc
// @Summary      Get link statistics
// @Description  Returns metadata (click count, expiry, active status), the 10 most recent click events, and per-day click counts for the last 7 days for the given slug.
// @Tags         links
// @Produce      json
// @Param        slug path     string true "URL slug" example("one-ring-to-rule")
// @Success      200  {object} StatsResponse
// @Failure      404  {object} ErrorResponse "slug not found, deleted, or not owned by caller"
// @Security     ApiKeyAuth
// @Router       /links/{slug}/stats [get]
func (lh *LinkHandler) GetStats(c *gin.Context) {

	// get the slug
	slug := c.Param("slug")

	// get the URLStats struct based on the slug
	urlStats, err := lh.store.GetStats(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stats not found"})
		return
	}

	callerKey := models.HashKey(middleware.ExtractBearerToken(c.GetHeader("Authorization")))
	if urlStats.APIKey != callerKey {
		// Same message as a nonexistent slug so this endpoint never leaks
		// which slugs exist versus which you don't own.
		c.JSON(http.StatusNotFound, gin.H{"error": "stats not found"})
		return
	}

	// Expired links intentionally still return stats: expiry stops redirects,
	// not the owner's analytics. Only owner-deleted (is_active = FALSE) links
	// are hidden, matching the list endpoint's filter — with the same message
	// as above so deletion state isn't leaked either.
	if !urlStats.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"error": "stats not found"})
		return
	}

	clicks, err := lh.store.GetClicks(slug, lh.cfg.RecentClicksLimit)
	if err != nil {
		log.Printf("failed to get clicks for slug %s: %v", slug, err)
		clicks = []models.Click{}
	}

	dailyClicks, err := lh.store.GetDailyClicks(slug, dailyClickDays)
	if err != nil {
		log.Printf("failed to get daily clicks for slug %s: %v", slug, err)
		dailyClicks = []models.DailyClickCount{}
	}

	// return the URL stats in JSON
	c.JSON(http.StatusOK, gin.H{
		"slug":          urlStats.Slug,
		"short_url":     lh.cfg.BaseURL + "/" + urlStats.Slug,
		"original":      urlStats.Original,
		"click_count":   urlStats.ClickCount,
		"created_at":    urlStats.CreatedAt,
		"expires_at":    urlStats.ExpiresAt,
		"is_active":     urlStats.IsActive,
		"recent_clicks": clicks,
		"daily_clicks":  dailyClicks,
	})
}
