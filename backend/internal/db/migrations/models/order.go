package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Order represents a buy or sell order.
type Order struct {
	ID                int64            `json:"id" db:"id"`
	ClientOrderID     string           `json:"client_order_id" db:"client_order_id"` // Traceability
	UserID            string           `json:"user_id" db:"user_id"`                 // UUID FK → users
	InstrumentID      int64            `json:"instrument_id" db:"instrument_id"`
	Side              OrderSide        `json:"side" db:"side"`             // BUY / SELL
	OrderType         OrderType        `json:"order_type" db:"order_type"` // LIMIT / MARKET
	Price             *decimal.Decimal `json:"price" db:"price"`           // NULL for MARKET orders
	Quantity          decimal.Decimal  `json:"quantity" db:"quantity"`
	RemainingQuantity decimal.Decimal  `json:"remaining_quantity" db:"remaining_quantity"`
	Status            OrderStatus      `json:"status" db:"status"`
	Fee               decimal.Decimal  `json:"fee" db:"fee"`
	FeeAsset          *string          `json:"fee_asset" db:"fee_asset"`
	IsSynthetic       bool             `json:"is_synthetic" db:"is_synthetic"` // System-generated liquidity
	PlacedAt          time.Time        `json:"placed_at" db:"placed_at"`       // Microsecond precision
	UpdatedAt         time.Time        `json:"updated_at" db:"updated_at"`
	CancelledAt       *time.Time       `json:"cancelled_at,omitempty" db:"cancelled_at"`
	CancelReason      *string          `json:"cancel_reason,omitempty" db:"cancel_reason"`
}
