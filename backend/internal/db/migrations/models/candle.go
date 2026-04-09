package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Candle represents OHLCV candlestick data for an instrument at a given interval.
type Candle struct {
	ID              int64           `json:"id" db:"id"`
	InstrumentID    int64           `json:"instrument_id" db:"instrument_id"`
	IntervalType    string          `json:"interval_type" db:"interval_type"`       // 1m, 5m, 1h, 1d
	IntervalSeconds int             `json:"interval_seconds" db:"interval_seconds"` // Numeric for custom queries
	OpenPrice       decimal.Decimal `json:"open_price" db:"open_price"`
	HighPrice       decimal.Decimal `json:"high_price" db:"high_price"`
	LowPrice        decimal.Decimal `json:"low_price" db:"low_price"`
	ClosePrice      decimal.Decimal `json:"close_price" db:"close_price"`
	Volume          decimal.Decimal `json:"volume" db:"volume"`
	StartTime       time.Time       `json:"start_time" db:"start_time"`
	EndTime         time.Time       `json:"end_time" db:"end_time"`
}
