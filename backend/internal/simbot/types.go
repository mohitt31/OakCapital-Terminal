package simbot

import "time"

// ──────────────────────────────────────────────────────────────────────────────
// Node types — must match the frontend's NodeType enum in alphaBot.ts exactly.
// ──────────────────────────────────────────────────────────────────────────────

type NodeType string

const (
	NodePriceFeed      NodeType = "priceFeed"
	NodeSMA            NodeType = "sma"
	NodeEMA            NodeType = "ema"
	NodeRSI            NodeType = "rsi"
	NodeMACD           NodeType = "macd"
	NodeBollingerBands NodeType = "bollingerBands"
	NodeCrossover      NodeType = "crossover"
	NodeThreshold      NodeType = "threshold"
	NodeAND            NodeType = "and"
	NodeOR             NodeType = "or"
	NodeMarketBuy      NodeType = "marketBuy"
	NodeMarketSell     NodeType = "marketSell"
	NodeStopLoss       NodeType = "stopLoss"
)

// ──────────────────────────────────────────────────────────────────────────────
// Graph primitives — the strategy graph sent from the frontend.
// These exactly mirror the BotNode / BotEdge / BotStrategy types.
// ──────────────────────────────────────────────────────────────────────────────

// BotNode is a single node in the strategy graph.
type BotNode struct {
	ID     string                 `json:"id"`
	Type   NodeType               `json:"type"`
	X      float64                `json:"x"`
	Y      float64                `json:"y"`
	Params map[string]interface{} `json:"params"`
	Label  string                 `json:"label"`
}

// BotEdge is a connection between two node ports.
type BotEdge struct {
	ID       string `json:"id"`
	FromNode string `json:"fromNode"`
	FromPort string `json:"fromPort"`
	ToNode   string `json:"toNode"`
	ToPort   string `json:"toPort"`
}

// StrategyGraph is the full strategy sent from the frontend.
type StrategyGraph struct {
	Nodes []BotNode `json:"nodes"`
	Edges []BotEdge `json:"edges"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Bot status and log types — sent back to the frontend.
// ──────────────────────────────────────────────────────────────────────────────

type BotStatus string
type BotMode string

const (
	StatusIdle    BotStatus = "idle"
	StatusRunning BotStatus = "running"
	StatusError   BotStatus = "error"
	StatusStopped BotStatus = "stopped"

	ModeSimulation BotMode = "simulation"
	ModeLive       BotMode = "live"
)

// BotLog is a single log entry produced by a running bot.
type BotLog struct {
	ID      int64  `json:"id"`
	Time    int64  `json:"time"`
	Message string `json:"message"`
	Type    string `json:"type"` // "info", "trade", "error"
}

// BotStateUpdate is pushed from a bot instance whenever its state changes.
type BotStateUpdate struct {
	BotID         string    `json:"bot_id"`
	Status        BotStatus `json:"status"`
	PnL           float64   `json:"pnl"`
	Symbol        string    `json:"symbol,omitempty"`
	Mode          BotMode   `json:"mode,omitempty"`
	StrategyLabel string    `json:"strategy_label,omitempty"`
	UserID        string    `json:"user_id,omitempty"`
	Log           *BotLog   `json:"log,omitempty"` // latest log entry, if any
}

// OrderAction is the output of the graph evaluator when an action node fires.
type OrderAction struct {
	Side     string // "buy" or "sell"
	Quantity float64
}

// ──────────────────────────────────────────────────────────────────────────────
// Exchange message types used by the simbot runtime.
// ──────────────────────────────────────────────────────────────────────────────

// Candle represents one OHLC price bar from the exchange.
type Candle struct {
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

// IncomingMessage is the envelope for all messages from the exchange.
type IncomingMessage struct {
	Type           string  `json:"type,omitempty"`
	Timestamp      int64   `json:"timestamp,omitempty"`
	ClientID       string  `json:"client_id,omitempty"`
	OrderID        string  `json:"order_id,omitempty"`
	Side           string  `json:"side,omitempty"`
	Status         string  `json:"status,omitempty"`
	Price          float64 `json:"price,omitempty"`
	Quantity       float64 `json:"quantity,omitempty"`
	BestBid        float64 `json:"best_bid,omitempty"`
	BestAsk        float64 `json:"best_ask,omitempty"`
	LastTradePrice float64 `json:"last_trade_price,omitempty"`

	// Alpha bot specific fields
	Prices         []Candle `json:"prices,omitempty"`
	PortfolioValue float64  `json:"portfolio_value,omitempty"`
	Position       float64  `json:"position,omitempty"`
}

// OutgoingOrder is what we send to the exchange.
type OutgoingOrder struct {
	Type     string  `json:"type"`
	ClientID string  `json:"client_id"`
	Symbol   string  `json:"symbol,omitempty"`
	OrderID  string  `json:"order_id,omitempty"`
	Side     string  `json:"side"`
	Quantity float64 `json:"quantity"`
	Mode     BotMode `json:"mode,omitempty"`
	UserID   string  `json:"user_id,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Config constants
// ──────────────────────────────────────────────────────────────────────────────

const (
	MaxPriceBuffer = 500
	TickInterval   = 500 * time.Millisecond
	MaxLogs        = 100
	OrderCooldown  = 1 * time.Second
	StopCooldown   = 3 * time.Second
)
