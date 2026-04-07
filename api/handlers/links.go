package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"log"
	"net/http"

	// local imports
	"github.com/wesley-lawson13/lembas-links/models"
)

type LinkHandler struct {
	store *models.URLStore
	redis *redis.Client
}

func NewLinkHandler(store *models.URLStore, redis *redis.Client) *LinkHandler {
	return &LinkHandler{store: store, redis: redis}
}

// POST /links function
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

	// create the url
	err = lh.store.CreateURL(slug, body.URL, body.APIKey)
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
		"short_url": "http://localhost:8080/" + slug,
		"original":  body.URL,
	})

}

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
