package models

import "time"

// PriceLevel represents a single [price, quantity] pair in the order book.
// Prices are in integer cents for precision.
type PriceLevel struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

// Side represents the order side (buy/sell).
type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

// OrderType represents the type of order.
type OrderType string

const (
	TypeLimit     OrderType = "LIMIT"
	TypeMarket    OrderType = "MARKET"
	TypeStop      OrderType = "STOP"
	TypeStopLimit OrderType = "STOP_LIMIT"
)

// OrderStatus represents the current state of an order.
type OrderStatus string

const (
	StatusOpen            OrderStatus = "OPEN"
	StatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	StatusFilled          OrderStatus = "FILLED"
	StatusCancelled       OrderStatus = "CANCELLED"
	StatusRejected        OrderStatus = "REJECTED"
)

// Timestamp utility for JSON uniformity.
func Now() int64 {
	return time.Now().UnixMilli()
}
