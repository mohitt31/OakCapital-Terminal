package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Position tracks holdings for each instrument within a portfolio.
// net_quantity > 0 = long, < 0 = short.
// unrealized_pnl is computed in-memory, NOT stored in DB (removed in v3).
type Position struct {
	ID                int64           `json:"id" db:"id"`
	PortfolioID       int64           `json:"portfolio_id" db:"portfolio_id"`
	InstrumentID      int64           `json:"instrument_id" db:"instrument_id"`
	NetQuantity       decimal.Decimal `json:"net_quantity" db:"net_quantity"`
	AverageEntryPrice decimal.Decimal `json:"average_entry_price" db:"average_entry_price"`
	UpdatedAt         time.Time       `json:"updated_at" db:"updated_at"`
}
