package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	// local imports
	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/models"
)

type SessionHandler struct {
	store *models.URLStore
	cfg   *config.Config
}

func NewSessionHandler(store *models.URLStore, cfg *config.Config) *SessionHandler {
	return &SessionHandler{store: store, cfg: cfg}
}

// CreateSession godoc
// @Summary      Mint an anonymous session key
// @Description  Generates a new random API key with no login/signup required. The key's expiry slides forward on every authenticated request, so an active visitor never loses access; only abandoned keys age out.
// @Tags         session
// @Produce      json
// @Success      201 {object} SessionResponse
// @Failure      500 {object} ErrorResponse "internal error (key generation or DB failure)"
// @Router       /session [post]
func (sh *SessionHandler) CreateSession(c *gin.Context) {

	ttl := time.Duration(sh.cfg.DefaultTTLDays) * 24 * time.Hour

	rawKey, err := sh.store.CreateKey(ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"api_key":    rawKey,
		"expires_at": time.Now().Add(ttl),
	})
}
