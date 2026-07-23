package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/models"
)

func APIKeyAuth(store *models.URLStore, cfg *config.Config) gin.HandlerFunc {

	ttl := time.Duration(cfg.DefaultTTLDays) * 24 * time.Hour

	return func(c *gin.Context) {

		header := c.GetHeader("Authorization")

		if header == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			c.Abort()
			return
		}

		key := ExtractBearerToken(header)

		err := store.ValidateKey(key, ttl)
		if err != nil {
			log.Printf("failed to validate api key: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired api key"})
			c.Abort()
			return
		}

		c.Next()
	}

}
