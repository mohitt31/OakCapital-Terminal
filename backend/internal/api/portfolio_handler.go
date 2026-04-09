package api

import (
	"errors"
	"net/http"
	"strings"

	"synthbull/internal/auth"
	"synthbull/internal/db"
	"synthbull/internal/portfolio"

	"github.com/gin-gonic/gin"
)

// PortfolioHandler exposes REST endpoints for the user-facing portfolio.
type PortfolioHandler struct {
	mgr        *portfolio.Manager
	userBotPnL *db.UserBotPnLRepo
}

func NewPortfolioHandler(mgr *portfolio.Manager, userBotPnL *db.UserBotPnLRepo) *PortfolioHandler {
	return &PortfolioHandler{mgr: mgr, userBotPnL: userBotPnL}
}

// GET /portfolio  — returns the caller's full portfolio snapshot.
func (h *PortfolioHandler) GetPortfolio(c *gin.Context) {
	userID := userIDFromContext(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	snap, err := h.mgr.GetSnapshot(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, portfolio.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session: user no longer exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": snap})
}

// GetBotPnLBreakdown returns persisted per-bot (BulBul) PnL rows for portfolio distribution.
// GET /api/v1/portfolio/bot-pnl
func (h *PortfolioHandler) GetBotPnLBreakdown(c *gin.Context) {
	userID := userIDFromContext(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if h.userBotPnL == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	rows, err := h.userBotPnL.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

// userIDFromContext extracts the user_id set by JWTMiddleware, or falls back
// to reading the Authorization header directly (for unauthenticated dev routes).
func userIDFromContext(c *gin.Context) string {
	if uid := c.GetString("user_id"); uid != "" {
		return uid
	}
	// Guest / dev: accept a plain userID in X-User-Id header for testing
	if uid := c.GetHeader("X-User-Id"); uid != "" {
		return uid
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Thin helpers that order_handler.go calls after each engine operation.
// They decode the JWT from the Authorization header to get userID without
// depending on Gin context (order_handler still uses net/http).
// ─────────────────────────────────────────────────────────────────────────────

// ExtractUserID reads a JWT bearer token from an Authorization header value
// and returns the embedded user_id claim. Returns "" on failure (anonymous).
func ExtractUserID(authHeader string, authSvc *auth.Service) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	claims, err := authSvc.ValidateToken(parts[1])
	if err != nil {
		return ""
	}
	return claims.UserID
}
