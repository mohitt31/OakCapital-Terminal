package portfolio

import "time"

// Position is an in-memory holding for one symbol inside a user's portfolio.
type Position struct {
	Symbol     string  // e.g. "BTCUSDT"
	Quantity   float64 // positive = long, negative = short
	AvgEntry   float64 // average cost basis
	MarkPrice  float64 // last traded price from engine
	PnL        float64 // (MarkPrice - AvgEntry) * Quantity
	UpdatedAt  time.Time
}

// Portfolio is the full in-memory state for one user.
type Portfolio struct {
	UserID       string
	PortfolioID  int64
	TotalCash    float64 // total cash (realised)
	AvailableCash float64 // cash not locked by open orders
	BlockedCash  float64 // cash locked by pending buy orders (escrow)
	Positions    map[string]*Position // symbol -> position
	UpdatedAt    time.Time
}

// Fill is a record of a completed trade, emitted to the WS.
type Fill struct {
	Symbol    string    `json:"symbol"`
	Side      string    `json:"side"`   // "buy" | "sell"
	Price     float64   `json:"price"`
	Qty       float64   `json:"qty"`
	PnL       float64   `json:"pnl"`   // realised PnL if closing position
	Timestamp time.Time `json:"timestamp"`
}

// Snapshot is the full portfolio state sent over HTTP / WS.
type Snapshot struct {
	UserID        string          `json:"user_id"`
	TotalCash     float64         `json:"total_cash"`
	AvailableCash float64         `json:"available_cash"`
	BlockedCash   float64         `json:"blocked_cash"`
	Positions     []PositionView  `json:"positions"`
	TotalPnL      float64         `json:"total_pnl"`
	Equity        float64         `json:"equity"` // available_cash + blocked_cash + holdings_value
	UpdatedAt     time.Time       `json:"updated_at"`
}

// PositionView is the JSON-safe view of a Position.
type PositionView struct {
	Symbol    string  `json:"symbol"`
	Quantity  float64 `json:"quantity"`
	AvgEntry  float64 `json:"avg_entry"`
	MarkPrice float64 `json:"mark_price"`
	PnL       float64 `json:"pnl"`
}
