package api

import "synthbull/internal/simbot"

// ──────────────────────────────────────────────────────────────────────────────
// Request / Response types for the simulation bot API endpoints.
// ──────────────────────────────────────────────────────────────────────────────

// StartBotRequest is the body for POST /api/v1/bot/start
type StartBotRequest struct {
	BotID        string               `json:"bot_id"`
	Symbol       string               `json:"symbol"`
	Mode         string               `json:"mode,omitempty"`
	StrategyName string               `json:"strategy_name,omitempty"`
	EvalInterval string               `json:"eval_interval,omitempty"`
	Strategy     simbot.StrategyGraph `json:"strategy"`
}

// StopBotRequest is the body for POST /api/v1/bot/stop
type StopBotRequest struct {
	BotID string `json:"bot_id"`
}

// BotStatusResponse returned by GET /api/v1/bot/status
type BotStatusResponse struct {
	BotID         string           `json:"bot_id"`
	Status        simbot.BotStatus `json:"status"`
	PnL           float64          `json:"pnl"`
	Symbol        string           `json:"symbol,omitempty"`
	Mode          string           `json:"mode,omitempty"`
	StrategyLabel string           `json:"strategy_label,omitempty"`
	Logs          []simbot.BotLog  `json:"logs,omitempty"`
}

// BotListItem is a single bot entry in the list response
type BotListItem struct {
	BotID         string           `json:"bot_id"`
	Status        simbot.BotStatus `json:"status"`
	PnL           float64          `json:"pnl"`
	Symbol        string           `json:"symbol,omitempty"`
	Mode          string           `json:"mode,omitempty"`
	StrategyLabel string           `json:"strategy_label,omitempty"`
}
