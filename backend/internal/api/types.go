package api

import (
	"synthbull/pkg/models"
	"time"
)

// ---------------------------------------------------------------------------
// REST API Response & Envelope
// ---------------------------------------------------------------------------

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    string      `json:"code,omitempty"`
}

// ---------------------------------------------------------------------------
// Order Requests (REST)
// ---------------------------------------------------------------------------

type AddLimitRequest struct {
	Symbol     string      `json:"symbol" binding:"required"`
	Side       models.Side `json:"side" binding:"required,oneof=buy sell"`
	Quantity   int64       `json:"quantity" binding:"required,gt=0"`
	LimitPrice int64       `json:"limit_price" binding:"required,gt=0"`
}

type CancelLimitRequest struct {
	Symbol  string `json:"symbol" binding:"required"`
	OrderID int64  `json:"order_id" binding:"required"`
}

type ModifyLimitRequest struct {
	Symbol     string `json:"symbol" binding:"required"`
	OrderID    int64  `json:"order_id" binding:"required"`
	Quantity   int64  `json:"quantity" binding:"required,gt=0"`
	LimitPrice int64  `json:"limit_price" binding:"required,gt=0"`
}

type MarketOrderRequest struct {
	Symbol   string      `json:"symbol" binding:"required"`
	Side     models.Side `json:"side" binding:"required,oneof=buy sell"`
	Quantity int64       `json:"quantity" binding:"required,gt=0"`
}

type AddStopRequest struct {
	Symbol    string      `json:"symbol" binding:"required"`
	Side      models.Side `json:"side" binding:"required,oneof=buy sell"`
	Quantity  int64       `json:"quantity" binding:"required,gt=0"`
	StopPrice int64       `json:"stop_price" binding:"required,gt=0"`
}

type CancelStopRequest struct {
	Symbol  string `json:"symbol" binding:"required"`
	OrderID int64  `json:"order_id" binding:"required"`
}

type ModifyStopRequest struct {
	Symbol    string `json:"symbol" binding:"required"`
	OrderID   int64  `json:"order_id" binding:"required"`
	Quantity  int64  `json:"quantity" binding:"required,gt=0"`
	StopPrice int64  `json:"stop_price" binding:"required,gt=0"`
}

type AddStopLimitRequest struct {
	Symbol     string      `json:"symbol" binding:"required"`
	Side       models.Side `json:"side" binding:"required,oneof=buy sell"`
	Quantity   int64       `json:"quantity" binding:"required,gt=0"`
	LimitPrice int64       `json:"limit_price" binding:"required,gt=0"`
	StopPrice  int64       `json:"stop_price" binding:"required,gt=0"`
}

type CancelStopLimitRequest struct {
	Symbol  string `json:"symbol" binding:"required"`
	OrderID int64  `json:"order_id" binding:"required"`
}

type ModifyStopLimitRequest struct {
	Symbol     string `json:"symbol" binding:"required"`
	OrderID    int64  `json:"order_id" binding:"required"`
	Quantity   int64  `json:"quantity" binding:"required,gt=0"`
	LimitPrice int64  `json:"limit_price" binding:"required,gt=0"`
	StopPrice  int64  `json:"stop_price" binding:"required,gt=0"`
}

// ---------------------------------------------------------------------------
// Market Data Responses (REST/Query)
// ---------------------------------------------------------------------------

type BookQueryRequest struct {
	Symbol string `json:"symbol" binding:"required"`
}

type BookInfoResponse struct {
	Symbol            string `json:"symbol"`
	BestBid           int64  `json:"best_bid"`
	BestAsk           int64  `json:"best_ask"`
	LastExecutedPrice int64  `json:"last_executed_price"`
	Timestamp         int64  `json:"timestamp"`
}

type BookDepthResponse struct {
	Symbol string              `json:"symbol"`
	Bids   []models.PriceLevel `json:"bids"`
	Asks   []models.PriceLevel `json:"asks"`
}

type HealthResponse struct {
	Status      string   `json:"status"`
	ActiveBooks []string `json:"active_books"`
	Timestamp   int64    `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// WebSocket Envelopes (Incoming/Outgoing)
// ---------------------------------------------------------------------------

type WSAction string

const (
	ActionSubscribe   WSAction = "subscribe"
	ActionUnsubscribe WSAction = "unsubscribe"
	ActionPing        WSAction = "ping"
)

type WSOutgoingType string

const (
	TypeTrade       WSOutgoingType = "trade"
	TypeOrderbook   WSOutgoingType = "orderbook"
	TypeCandle      WSOutgoingType = "candle"
	TypeTicker      WSOutgoingType = "ticker"
	TypeDepth       WSOutgoingType = "depth"
	TypeOrderUpdate WSOutgoingType = "order_update"
	TypePortfolio   WSOutgoingType = "portfolio"
	TypeAlert       WSOutgoingType = "alert"
	TypeBotStatus   WSOutgoingType = "bot_status"
	TypeError       WSOutgoingType = "error"
	TypeSuccess     WSOutgoingType = "success"
)

// Incoming (from Client to Server)
type WSRequest struct {
	Action  WSAction `json:"action"`
	Symbols []string `json:"symbols"`
	Event   string   `json:"event"` // e.g. "trade", "ticker", "all"
}

// Outgoing (from Server to Client)
type WSResponse struct {
	Type      WSOutgoingType `json:"type"`
	Symbol    string         `json:"symbol,omitempty"`
	Timestamp int64          `json:"timestamp"`
	// Data can be eventbus.TradeEvent, eventbus.DepthEvent, etc.
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}

// ---------------------------------------------------------------------------
// Market Simulation (GBM Ticks)
// ---------------------------------------------------------------------------
// Moved to eventbus

// ---------------------------------------------------------------------------
// Bot Strategy & Management
// ---------------------------------------------------------------------------

type BotStatus string

const (
	BotRunning BotStatus = "running"
	BotStopped BotStatus = "stopped"
	BotErrored BotStatus = "error"
)

type BotConfig struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Symbol    string                 `json:"symbol"`
	Strategy  string                 `json:"strategy"`
	Params    map[string]interface{} `json:"params"`
	Status    BotStatus              `json:"status"`
	CreatedAt time.Time              `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Portfolio & User State
// ---------------------------------------------------------------------------

type Position struct {
	Symbol       string `json:"symbol"`
	Quantity     int64  `json:"quantity"`
	AveragePrice int64  `json:"average_price"`
	RealizedPnL  int64  `json:"realized_pnl"`
}

type Portfolio struct {
	UserID    string              `json:"user_id"`
	Cash      int64               `json:"cash"`
	Positions map[string]Position `json:"positions"`
	UpdatedAt time.Time           `json:"updated_at"`
}
