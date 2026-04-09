package api

import (
	"net/http"
	"strings"

	"synthbull/internal/db"

	"github.com/gin-gonic/gin"
)

// WatchlistHandler exposes REST endpoints for per-user stock watchlists.
type WatchlistHandler struct {
	repo *db.WatchlistRepo
}

// NewWatchlistHandler creates a WatchlistHandler.
func NewWatchlistHandler(repo *db.WatchlistRepo) *WatchlistHandler {
	return &WatchlistHandler{repo: repo}
}

// POST /api/v1/watchlist
// Body: {"symbol": "RELIANCE"}
// Adds a symbol to the authenticated user's watchlist.
func (h *WatchlistHandler) Add(c *gin.Context) {
	userID := userIDFromContext(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var body struct {
		Symbol string `json:"symbol"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	body.Symbol = strings.ToUpper(strings.TrimSpace(body.Symbol))
	if body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	if err := h.repo.Add(c.Request.Context(), userID, body.Symbol); err != nil {
		if strings.Contains(err.Error(), "unknown symbol") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "added to watchlist", "symbol": body.Symbol})
}

// DELETE /api/v1/watchlist
// Body: {"symbol": "RELIANCE"}
// Removes a symbol from the authenticated user's watchlist.
func (h *WatchlistHandler) Remove(c *gin.Context) {
	userID := userIDFromContext(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var body struct {
		Symbol string `json:"symbol"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	body.Symbol = strings.ToUpper(strings.TrimSpace(body.Symbol))
	if body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	if err := h.repo.Remove(c.Request.Context(), userID, body.Symbol); err != nil {
		if strings.Contains(err.Error(), "unknown symbol") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "removed from watchlist", "symbol": body.Symbol})
}

// GET /api/v1/watchlist
// Returns the authenticated user's full watchlist.
func (h *WatchlistHandler) List(c *gin.Context) {
	userID := userIDFromContext(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	items, err := h.repo.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}
