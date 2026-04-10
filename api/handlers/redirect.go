package handlers

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

// Redirect godoc
// @Summary      Redirect to original URL
// @Description  Resolves a slug to its original URL (Redis cache first, then DB) and issues a 302 redirect. Click metadata is recorded asynchronously. Returns 410 if the link has expired.
// @Tags         redirect
// @Param        slug path string true "URL slug" example("one-ring-to-rule")
// @Success      302  {string} string "Location header set to the original URL"
// @Failure      404  {object} ErrorResponse "slug not found or inactive"
// @Failure      410  {object} ErrorResponse "link has expired"
// @Router       /{slug} [get]
func (lh *LinkHandler) Redirect(c *gin.Context) {

	// get the context
	slug := c.Param("slug")
	referrer := c.GetHeader("Referer")
	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	// check redis for the slug
	cached, err := lh.redis.Get(c, slug).Result()
	if err == nil {

		lh.asyncRecordClick(slug, referrer, userAgent, ip)

		c.Redirect(http.StatusFound, cached)
		return
	}

	// check db for the slug
	original, err := lh.store.GetURL(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "slug not found"})
		return
	}

	// expired url
	if time.Now().After(original.ExpiresAt) {

		if err = lh.store.DeleteURL(slug); err != nil {
			log.Printf("failed to delete slug %s: %v", slug, err)
		}

		c.JSON(http.StatusGone, gin.H{"error": "url expired"})
		return
	}

	// check if the URL is still active
	if !original.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"error": "url not active"})
		return
	}

	// cache in redis
	ttl := time.Until(original.ExpiresAt)
	err = lh.redis.Set(c, slug, original.Original, ttl).Err()
	if err != nil {
		log.Printf("failed to cache slug %s: %v", slug, err)
	}

	lh.asyncRecordClick(slug, referrer, userAgent, ip)

	// redirect
	c.Redirect(http.StatusFound, original.Original)
}

func (lh *LinkHandler) asyncRecordClick(slug, referrer, userAgent, ipAddress string) {

	go func() {

		if err := lh.store.RecordClick(slug, referrer, userAgent, ipAddress); err != nil {
			log.Printf("failed to record click for slug %s: %v", slug, err)
		}

		if err := lh.store.IncrementClickCount(slug); err != nil {
			log.Printf("failed to increment click count for slug %s: %v", slug, err)
		}
	}()
}
