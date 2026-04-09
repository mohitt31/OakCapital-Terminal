package models

import "github.com/shopspring/decimal"

// Instrument represents a tradable instrument (e.g., BTC-USD).
type Instrument struct {
	ID            int64           `json:"id" db:"id"`
	Symbol        string          `json:"symbol" db:"symbol"` // e.g. "BTC-USD"
	BaseCurrency  string          `json:"base_currency" db:"base_currency"`
	QuoteCurrency string          `json:"quote_currency" db:"quote_currency"`
	TickSize      decimal.Decimal `json:"tick_size" db:"tick_size"` // Minimum price increment
	LotSize       decimal.Decimal `json:"lot_size" db:"lot_size"`   // Minimum quantity increment
	IsActive      bool            `json:"is_active" db:"is_active"`
}
