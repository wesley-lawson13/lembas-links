package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wesley-lawson13/lembas-links/models"
)

func APIKeyAuth(store *models.URLStore) gin.HandlerFunc {

	return func(c *gin.Context) {

		key := c.GetHeader("Authorization")

		if key == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			c.Abort()
			return
		}

		err := store.ValidateKey(key)
		if err != nil {
			log.Printf("failed to validate api key: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "api key not found"})
			c.Abort()
			return
		}

		c.Next()
	}

}
