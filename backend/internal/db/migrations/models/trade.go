package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Trade represents an executed match between a buy and sell order.
type Trade struct {
	ID                    int64           `json:"id" db:"id"`
	InstrumentID          int64           `json:"instrument_id" db:"instrument_id"`
	BuyOrderID            int64           `json:"buy_order_id" db:"buy_order_id"`
	SellOrderID           int64           `json:"sell_order_id" db:"sell_order_id"`
	Price                 decimal.Decimal `json:"price" db:"price"`
	Quantity              decimal.Decimal `json:"quantity" db:"quantity"`
	SideThatTookLiquidity OrderSide       `json:"side_that_took_liquidity" db:"side_that_took_liquidity"` // Taker
	ExecutedAt            time.Time       `json:"executed_at" db:"executed_at"`                           // Microsecond precision
}
