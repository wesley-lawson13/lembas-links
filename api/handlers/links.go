package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	// local imports
	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/middleware"
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
// @Failure      400  {object} ErrorResponse "missing body, or url is empty, malformed, non-http(s), or too long"
// @Failure      500  {object} ErrorResponse "internal error (slug pool exhausted, DB failure)"
// @Security     ApiKeyAuth
// @Router       /links [post]
func (lh *LinkHandler) CreateLink(c *gin.Context) {

	var body struct {
		URL string `json:"url"`
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

	// reject anything we shouldn't redirect to (non-http(s) schemes, no host,
	// over-length) so this endpoint can't be used as an open redirector
	if err := validateTargetURL(body.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// create the expires at value
	expiresAt := time.Now().Add(time.Duration(lh.cfg.DefaultTTLDays) * 24 * time.Hour)

	// ownership comes from the authenticated caller, not the request body
	ownerKey := models.HashKey(middleware.ExtractBearerToken(c.GetHeader("Authorization")))

	// atomically claim a slug and create the url in one step (picks the
	// least-used quote and bumps its use_count internally)
	slug, err := lh.store.AllocateURL(body.URL, ownerKey, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create link"})
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
// @Description  Soft-deletes the link (sets is_active = false) and evicts it from the Redis cache. The slug is not returned to the pool. Only the link's owner may delete it.
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

	url, err := lh.store.GetURL(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "slug not found"})
		return
	}

	callerKey := models.HashKey(middleware.ExtractBearerToken(c.GetHeader("Authorization")))
	if url.APIKey != callerKey {
		// Same message as a nonexistent slug so this endpoint never leaks
		// which slugs exist versus which you don't own.
		c.JSON(http.StatusNotFound, gin.H{"error": "slug not found"})
		return
	}

	// call the delete function and check for errors
	err = lh.store.DeleteURL(slug)
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

// ListLinks godoc
// @Summary      List the caller's own links
// @Description  Returns a summary of every active link owned by the authenticated API key.
// @Tags         links
// @Produce      json
// @Success      200  {object} ListLinksResponse
// @Security     ApiKeyAuth
// @Router       /links [get]
func (lh *LinkHandler) ListLinks(c *gin.Context) {

	rawKey := middleware.ExtractBearerToken(c.GetHeader("Authorization"))

	urls, err := lh.store.ListURLsByAPIKey(rawKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list links"})
		return
	}

	links := make([]LinkSummary, 0, len(urls))
	for _, url := range urls {
		links = append(links, LinkSummary{
			Slug:       url.Slug,
			ShortURL:   lh.cfg.BaseURL + "/" + url.Slug,
			Original:   url.Original,
			ClickCount: url.ClickCount,
			CreatedAt:  url.CreatedAt,
			ExpiresAt:  url.ExpiresAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"links": links})
}
