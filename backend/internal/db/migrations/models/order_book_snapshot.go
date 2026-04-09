package models

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// PriceLevel represents a single [price, quantity] pair in the order book.
type PriceLevel struct {
	Price    decimal.Decimal `json:"price"`
	Quantity decimal.Decimal `json:"quantity"`
}

// OrderBookSnapshot stores a point-in-time snapshot of the order book.
type OrderBookSnapshot struct {
	ID           int64            `json:"id" db:"id"`
	InstrumentID int64            `json:"instrument_id" db:"instrument_id"`
	Bids         json.RawMessage  `json:"bids" db:"bids"` // JSONB: list of [price, quantity]
	Asks         json.RawMessage  `json:"asks" db:"asks"` // JSONB: list of [price, quantity]
	MidPrice     *decimal.Decimal `json:"mid_price" db:"mid_price"`
	Spread       *decimal.Decimal `json:"spread" db:"spread"`
	SnappedAt    time.Time        `json:"snapped_at" db:"snapped_at"`
}
