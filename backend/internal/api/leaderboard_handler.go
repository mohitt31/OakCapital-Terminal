package api

import (
	"net/http"
	"strings"

	"synthbull/internal/db"

	"github.com/gin-gonic/gin"
)

type LeaderboardHandler struct {
	repo *db.LeaderboardRepo
}

func NewLeaderboardHandler(repo *db.LeaderboardRepo) *LeaderboardHandler {
	return &LeaderboardHandler{repo: repo}
}

type leaderboardPublishReq struct {
	BotID         string `json:"bot_id" binding:"required"`
	IsPublic      bool   `json:"is_public"`
	ShareStrategy bool   `json:"share_strategy"`
}

// GET /api/v1/leaderboard?include_public=true&scope=weekly
func (h *LeaderboardHandler) Get(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if h.repo == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	includePublic := strings.ToLower(c.DefaultQuery("include_public", "true")) != "false"
	scope := strings.ToLower(c.DefaultQuery("scope", "weekly"))
	weeklyOnly := scope != "all"

	rows, err := h.repo.List(c.Request.Context(), userID, includePublic, weeklyOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rows,
	})
}

// POST /api/v1/leaderboard/publish
func (h *LeaderboardHandler) Publish(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "leaderboard unavailable"})
		return
	}
	var req leaderboardPublishReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	owns, err := h.repo.UserOwnsBot(c.Request.Context(), userID, req.BotID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !owns {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only change publish settings for your own bots"})
		return
	}
	if err := h.repo.UpsertPublishSettings(c.Request.Context(), db.LeaderboardPublishRequest{
		UserID:        userID,
		BotID:         req.BotID,
		IsPublic:      req.IsPublic,
		ShareStrategy: req.ShareStrategy,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"bot_id":         req.BotID,
			"is_public":      req.IsPublic,
			"share_strategy": req.ShareStrategy,
		},
	})
}
