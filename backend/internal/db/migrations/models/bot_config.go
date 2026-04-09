package models

import (
	"encoding/json"
	"time"
)

// BotConfig holds the configuration for an automated trading bot.
type BotConfig struct {
	ID             int64           `json:"id" db:"id"`
	UserID         string          `json:"user_id" db:"user_id"` // UUID FK → users
	InstrumentID   int64           `json:"instrument_id" db:"instrument_id"`
	BotType        string          `json:"bot_type" db:"bot_type"` // e.g. MARKET_MAKER
	Status         BotStatus       `json:"status" db:"status"`
	StrategyParams json.RawMessage `json:"strategy_params" db:"strategy_params"` // JSONB
	RiskLimit      json.RawMessage `json:"risk_limit" db:"risk_limit"`           // JSONB
	LastRunAt      *time.Time      `json:"last_run_at,omitempty" db:"last_run_at"`
	ErrorLog       *string         `json:"error_log,omitempty" db:"error_log"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}
