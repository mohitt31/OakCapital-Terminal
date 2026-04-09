package engine

import "fmt"

// Status maps engine status codes.
type Status int

const (
	StatusOK         Status = 0
	StatusNullHandle Status = 1
	StatusInvalidArg Status = 2
	StatusNotFound   Status = 3
	StatusInternal   Status = 100
)

const (
	SideSell = 0
	SideBuy  = 1
)

func (s Status) Error() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusNullHandle:
		return "null engine handle"
	case StatusInvalidArg:
		return "invalid argument"
	case StatusNotFound:
		return "order not found"
	case StatusInternal:
		return "internal engine error"
	default:
		return fmt.Sprintf("unknown engine status %d", int(s))
	}
}

// PriceLevel represents a single price level in the order book depth.
type PriceLevel struct {
	Price  int `json:"price"`
	Volume int `json:"volume"`
}

// LevelChange represents a change to a price level after an order operation.
type LevelChange struct {
	Price  int `json:"price"`
	Volume int `json:"volume"`
	Side   int `json:"side"`
}

type Trade struct {
	Price        int   `json:"price"`
	Qty          int   `json:"qty"`
	MakerOrderID int   `json:"maker_order_id"`
	TakerOrderID int   `json:"taker_order_id"`
	TimestampUnixNano int64 `json:"timestamp_unix_nano"`
}

// OrderResult is returned after every order operation.
type OrderResult struct {
	Changes []LevelChange
	Trades  []Trade
}

// Depth is a snapshot of the order book.
type Depth struct {
	BestBid   int          `json:"bestBid"`
	BestAsk   int          `json:"bestAsk"`
	LastPrice int          `json:"lastPrice"`
	Bids      []PriceLevel `json:"bids"`
	Asks      []PriceLevel `json:"asks"`
}
