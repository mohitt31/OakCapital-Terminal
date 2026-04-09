package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"synthbull/internal/db"
	"synthbull/internal/simbot"

	"github.com/gin-gonic/gin"
)

// ──────────────────────────────────────────────────────────────────────────────
// SimHandler — HTTP handlers for the simulation bot API.
//
// POST /api/v1/simbot/start   → Create & start a simulation bot
// POST /api/v1/simbot/stop    → Stop a running bot
// GET  /api/v1/simbot/status  → Get current bot state
// GET  /api/v1/simbot/list    → List current user's bot instances
// ──────────────────────────────────────────────────────────────────────────────

type SimHandler struct {
	manager    *simbot.BotManager
	userBotPnL *db.UserBotPnLRepo
}

// NewSimHandler creates a new simulation bot handler. userBotPnL may be nil.
func NewSimHandler(manager *simbot.BotManager, userBotPnL *db.UserBotPnLRepo) *SimHandler {
	return &SimHandler{manager: manager, userBotPnL: userBotPnL}
}

func (h *SimHandler) persistBotPnL(ctx context.Context, userID string, st simbot.BotStateUpdate) {
	if h.userBotPnL == nil || userID == "" {
		return
	}
	_ = h.userBotPnL.Upsert(ctx, db.UserBotPnLRow{
		UserID:       userID,
		BotID:        st.BotID,
		StrategyName: st.StrategyLabel,
		Symbol:       st.Symbol,
		Mode:         string(st.Mode),
		PnL:          st.PnL,
		Status:       string(st.Status),
	})
}

// StartFlagshipBot starts a prebuilt advanced alpha strategy (separate from GUI builder).
//
// Optional body field: `strategy_name` selects which strategy to run.
// Supported values: flagship_v2 (default), bollinger_mean_reversion, macd_momentum,
// rsi_reversal, fast_ema_trend, macd_bollinger_breakout.
func (h *SimHandler) StartFlagshipBot(c *gin.Context) {
	var req struct {
		BotID        string `json:"bot_id"`
		Symbol       string `json:"symbol" binding:"required"`
		Mode         string `json:"mode,omitempty"`
		StrategyName string `json:"strategy_name,omitempty"`
		EvalInterval string `json:"eval_interval,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Resolve strategy — defaults to flagship_v2 when empty
	strategyGraph, strategyLabel := simbot.GetStrategy(simbot.StrategyName(req.StrategyName))

	if req.BotID == "" {
		suffix := req.StrategyName
		if suffix == "" {
			suffix = "flagship"
		}
		req.BotID = suffix + "_" + strings.ToLower(req.Symbol) + "_" + time.Now().UTC().Format("150405")
	}
	mode := simbot.ModeSimulation
	if req.Mode == string(simbot.ModeLive) {
		mode = simbot.ModeLive
	}
	evalInterval := simbot.ParseEvalInterval(req.EvalInterval, time.Minute)
	userID := c.GetString("user_id")
	if err := h.manager.StartBot(req.BotID, req.Symbol, strategyGraph, mode, userID, strategyLabel, evalInterval); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "flagship bot started",
		"data": gin.H{
			"bot_id":         req.BotID,
			"symbol":         req.Symbol,
			"mode":           mode,
			"status":         "running",
			"strategy_name":  req.StrategyName,
			"strategy_label": strategyLabel,
			"eval_interval":  simbot.FormatEvalInterval(evalInterval),
		},
	})
}

// StartBot handles POST /api/v1/simbot/start
// Body: { "bot_id": "...", "strategy": { "nodes": [...], "edges": [...] } }
func (h *SimHandler) StartBot(c *gin.Context) {
	var req StartBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	if req.BotID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}
	if req.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	if len(req.Strategy.Nodes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy must have at least one node"})
		return
	}

	mode := simbot.ModeSimulation
	if req.Mode == string(simbot.ModeLive) {
		mode = simbot.ModeLive
	}
	evalInterval := simbot.ParseEvalInterval(req.EvalInterval, 1*time.Second)
	userID := c.GetString("user_id")
	label := strings.TrimSpace(req.StrategyName)
	if label == "" {
		label = "BulBul"
	}

	if err := h.manager.StartBot(req.BotID, req.Symbol, req.Strategy, mode, userID, label, evalInterval); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "bot started",
		"data": map[string]interface{}{
			"bot_id":        req.BotID,
			"symbol":        req.Symbol,
			"status":        "running",
			"mode":          mode,
			"nodes":         len(req.Strategy.Nodes),
			"edges":         len(req.Strategy.Edges),
			"eval_interval": simbot.FormatEvalInterval(evalInterval),
		},
	})
}

// StopBot handles POST /api/v1/simbot/stop
// Body: { "bot_id": "..." }
func (h *SimHandler) StopBot(c *gin.Context) {
	var req StopBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	if req.BotID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}

	userID := c.GetString("user_id")
	if err := h.manager.StopBotForUser(req.BotID, userID); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "bot stopped",
		"data": map[string]interface{}{
			"bot_id": req.BotID,
			"status": "stopped",
		},
	})
}

// GetBotStatus handles GET /api/v1/simbot/status?bot_id=...
func (h *SimHandler) GetBotStatus(c *gin.Context) {
	botID := c.Query("bot_id")
	if botID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id query parameter is required"})
		return
	}

	userID := c.GetString("user_id")
	state, logs, err := h.manager.GetStatusForUser(botID, userID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.persistBotPnL(c.Request.Context(), userID, state)

	c.JSON(http.StatusOK, gin.H{
		"message": "bot status retrieved",
		"data": BotStatusResponse{
			BotID:         state.BotID,
			Status:        state.Status,
			PnL:           state.PnL,
			Symbol:        state.Symbol,
			Mode:          string(state.Mode),
			StrategyLabel: state.StrategyLabel,
			Logs:          logs,
		},
	})
}

// ListBots handles GET /api/v1/simbot/list — only bots owned by the authenticated user.
func (h *SimHandler) ListBots(c *gin.Context) {
	userID := c.GetString("user_id")
	states := h.manager.ListBotsForUser(userID)
	items := make([]BotListItem, len(states))
	for i, s := range states {
		items[i] = BotListItem{
			BotID:         s.BotID,
			Status:        s.Status,
			PnL:           s.PnL,
			Symbol:        s.Symbol,
			Mode:          string(s.Mode),
			StrategyLabel: s.StrategyLabel,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "bots listed",
		"data": map[string]interface{}{
			"count": len(items),
			"bots":  items,
		},
	})
}
