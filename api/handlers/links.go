package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	// local imports
	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/models"
)

type LinkHandler struct {
	store *models.URLStore
	redis *redis.Client
	cfg   *config.Config
}

func NewLinkHandler(store *models.URLStore, redis *redis.Client, cfg *config.Config) *LinkHandler {
	return &LinkHandler{store: store, redis: redis, cfg: cfg}
}

// CreateLink godoc
// @Summary      Create a shortened link
// @Description  Selects an available LOTR-themed slug from the quote pool and maps it to the provided URL. The link expires after the configured TTL (default 30 days).
// @Tags         links
// @Accept       json
// @Produce      json
// @Param        body body     CreateLinkRequest true "URL to shorten"
// @Success      201  {object} CreateLinkResponse
// @Failure      400  {object} ErrorResponse "missing or invalid request body"
// @Failure      500  {object} ErrorResponse "internal error (slug pool exhausted, DB failure)"
// @Security     ApiKeyAuth
// @Router       /links [post]
func (lh *LinkHandler) CreateLink(c *gin.Context) {

	var body struct {
		URL    string `json:"url"`
		APIKey string `json:"api_key"`
	}

	// map contect from gin into the HTTP response body struct
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body request"})
		return
	}

	if body.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	// get a slug for the long url
	slug, err := lh.store.GetSlug()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get slug"})
		return
	}

	// create the expires at value
	expiresAt := time.Now().Add(time.Duration(lh.cfg.DefaultTTLDays) * 24 * time.Hour)

	// create the url
	err = lh.store.CreateURL(slug, body.URL, body.APIKey, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create link"})
		return
	}

	// Increment the quote's use_count
	err = lh.store.IncrementUseCount(slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update slug"})
		return
	}

	// success - 201 return type
	c.JSON(http.StatusCreated, gin.H{
		"slug":      slug,
		"short_url": lh.cfg.BaseURL + "/" + slug,
		"original":  body.URL,
	})

}

// DeleteLink godoc
// @Summary      Delete a shortened link
// @Description  Soft-deletes the link (sets is_active = false) and evicts it from the Redis cache. The slug is not returned to the pool.
// @Tags         links
// @Produce      json
// @Param        slug path     string true "URL slug" example("one-ring-to-rule")
// @Success      204
// @Failure      404  {object} ErrorResponse "slug not found"
// @Security     ApiKeyAuth
// @Router       /links/{slug} [delete]
func (lh *LinkHandler) DeleteLink(c *gin.Context) {

	// get the slug
	slug := c.Param("slug")

	// call the delete function and check for errors
	err := lh.store.DeleteURL(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "slug not found"})
		return
	}

	// delete from the redis cache
	err = lh.redis.Del(c, slug).Err()
	if err != nil {
		log.Printf("failed to delete slug %s from cache: %v", slug, err)
	}

	// only need to return the status
	c.Status(http.StatusNoContent)
}
