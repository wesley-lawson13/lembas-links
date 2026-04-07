package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (lh *LinkHandler) GetStats(c *gin.Context) {

	// get the slug
	slug := c.Param("slug")

	// get the URLStats struct based on the slug
	urlStats, err := lh.store.GetStats(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stats not found"})
		return
	}

	if !urlStats.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}

	// return the URL stats in JSON
	c.JSON(http.StatusOK, gin.H{
		"slug":        urlStats.Slug,
		"original":    urlStats.Original,
		"click_count": urlStats.ClickCount,
		"created_at":  urlStats.CreatedAt,
		"expires_at":  urlStats.ExpiresAt,
		"is_active":   urlStats.IsActive,
	})
}
