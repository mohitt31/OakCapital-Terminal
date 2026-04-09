package models

// ============================================================
// Shared Enums / Type Constants
// ============================================================

// OrderSide represents the side of an order.
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderType represents the type of an order.
type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

// OrderStatus represents the lifecycle status of an order.
type OrderStatus string

const (
	OrderStatusOpen      OrderStatus = "OPEN"
	OrderStatusFilled    OrderStatus = "FILLED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
	OrderStatusRejected  OrderStatus = "REJECTED"
)

// SimulationType represents the market simulation strategy.
type SimulationType string

const (
	SimulationTypeGBM    SimulationType = "GBM"
	SimulationTypeSine   SimulationType = "SINE"
	SimulationTypeReplay SimulationType = "REPLAY"
)

// BotStatus represents the running state of a bot.
type BotStatus string

const (
	BotStatusRunning BotStatus = "RUNNING"
	BotStatusStopped BotStatus = "STOPPED"
	BotStatusPaused  BotStatus = "PAUSED"
)

// Severity represents the severity level of a system event.
type Severity string

const (
	SeverityInfo  Severity = "INFO"
	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
)
