package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Portfolio tracks the cash side of a user account — the wallet for the simulator.
type Portfolio struct {
	ID            int64           `json:"id" db:"id"`
	UserID        string          `json:"user_id" db:"user_id"` // UUID FK → users
	Name          string          `json:"name" db:"name"`
	TotalCash     decimal.Decimal `json:"total_cash" db:"total_cash"`
	AvailableCash decimal.Decimal `json:"available_cash" db:"available_cash"`
	BlockedCash   decimal.Decimal `json:"blocked_cash" db:"blocked_cash"`   // Locked by open orders
	MarginLocked  decimal.Decimal `json:"margin_locked" db:"margin_locked"` // Locked for short positions
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}
