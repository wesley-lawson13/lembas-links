package handlers

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

func (lh *LinkHandler) Redirect(c *gin.Context) {

	// get the slug
	slug := c.Param("slug")

	// check redis for the slug
	cached, err := lh.redis.Get(c, slug).Result()
	if err == nil {

		if err = lh.store.IncrementClickCount(slug); err != nil {
			log.Printf("failed to increment click_count for slug %s: %v", slug, err)
		}

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

	// increment click count
	if err = lh.store.IncrementClickCount(slug); err != nil {
		log.Printf("failed to increment click_count for slug %s: %v", slug, err)
	}

	// redirect
	c.Redirect(http.StatusFound, original.Original)
}
