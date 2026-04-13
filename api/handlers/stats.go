package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wesley-lawson13/lembas-links/models"
)

// GetStats godoc
// @Summary      Get link statistics
// @Description  Returns metadata (click count, expiry, active status) and the 10 most recent click events for the given slug.
// @Tags         links
// @Produce      json
// @Param        slug path     string true "URL slug" example("one-ring-to-rule")
// @Success      200  {object} StatsResponse
// @Failure      404  {object} ErrorResponse "slug not found or expired"
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

	if !urlStats.IsActive || time.Now().After(urlStats.ExpiresAt) {
		c.JSON(http.StatusNotFound, gin.H{"error": "url expired"})
		return
	}

	clicks, err := lh.store.GetClicks(slug, lh.cfg.RecentClicksLimit)
	if err != nil {
		log.Printf("failed to get clicks for slug %s: %v", slug, err)
		clicks = []models.Click{}
	}

	// return the URL stats in JSON
	c.JSON(http.StatusOK, gin.H{
		"slug":          urlStats.Slug,
		"original":      urlStats.Original,
		"click_count":   urlStats.ClickCount,
		"created_at":    urlStats.CreatedAt,
		"expires_at":    urlStats.ExpiresAt,
		"is_active":     urlStats.IsActive,
		"recent_clicks": clicks,
	})
}
