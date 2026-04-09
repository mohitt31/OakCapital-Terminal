package eventbus

import (
	"time"

	"synthbull/pkg/models"
)

type TradeEvent struct {
	Symbol       string      `json:"symbol"`
	Price        int64       `json:"price"`
	Quantity     int64       `json:"quantity"`
	MakerOrderID int64       `json:"maker_order_id"`
	TakerOrderID int64       `json:"taker_order_id"`
	TakerSide    models.Side `json:"taker_side"`
	ExecutedAt   time.Time   `json:"executed_at"`
}

type OrderUpdateEvent struct {
	OrderID      int64              `json:"order_id"`
	ClientID     string             `json:"client_id,omitempty"`
	UserID       string             `json:"user_id"`
	Symbol       string             `json:"symbol"`
	Side         models.Side        `json:"side"`
	Type         models.OrderType   `json:"type"`
	Status       models.OrderStatus `json:"status"`
	Quantity     int64              `json:"quantity"`
	FilledQty    int64              `json:"filled_qty"`
	RemainingQty int64              `json:"remaining_qty"`
	AvgPrice     int64              `json:"avg_price"`
	LimitPrice   int64              `json:"limit_price,omitempty"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type DepthEvent struct {
	Symbol     string              `json:"symbol"`
	Bids       []models.PriceLevel `json:"bids"`
	Asks       []models.PriceLevel `json:"asks"`
	IsSnapshot bool                `json:"is_snapshot"`
	Timestamp  time.Time           `json:"timestamp"`
}

type TickerEvent struct {
	Symbol    string    `json:"symbol"`
	BestBid   int64     `json:"best_bid"`
	BestAsk   int64     `json:"best_ask"`
	LastPrice int64     `json:"last_price"`
	Timestamp time.Time `json:"timestamp"`
}

type CandleEvent struct {
	Symbol    string    `json:"symbol"`
	Interval  string    `json:"interval"`
	Open      int64     `json:"open"`
	High      int64     `json:"high"`
	Low       int64     `json:"low"`
	Close     int64     `json:"close"`
	Volume    int64     `json:"volume"`
	IsClosed  bool      `json:"is_closed"`
	Timestamp time.Time `json:"timestamp"`
}

type GBMTickEvent struct {
	Symbol    string    `json:"symbol"`
	BasePrice int64     `json:"base_price"`
	Timestamp time.Time `json:"timestamp"`
}

type PortfolioPosition struct {
	Symbol       string `json:"symbol"`
	Quantity     int64  `json:"quantity"`
	AveragePrice int64  `json:"average_price"`
	RealizedPnL  int64  `json:"realized_pnl"`
}

type PortfolioEvent struct {
	UserID    string              `json:"user_id"`
	Cash      int64               `json:"cash"`
	Equity    int64               `json:"equity"`
	PnL       int64               `json:"pnl"`
	Positions []PortfolioPosition `json:"positions"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type AlertEvent struct {
	UserID    string    `json:"user_id"`
	AlertType string    `json:"alert_type"`
	Symbol    string    `json:"symbol,omitempty"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	CreatedAt time.Time `json:"created_at"`
}

type BotStatusEvent struct {
	BotID     string    `json:"bot_id"`
	ClientID  string    `json:"client_id"`
	UserID    string    `json:"user_id,omitempty"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type HealthEvent struct {
	ServiceName string    `json:"service_name"`
	Status      string    `json:"status"`
	LatencyMs   int       `json:"latency_ms"`
	Details     string    `json:"details,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

type ErrorEvent struct {
	ServiceName string    `json:"service_name"`
	ErrorCode   string    `json:"error_code,omitempty"`
	Message     string    `json:"message"`
	Details     string    `json:"details,omitempty"`
	Severity    string    `json:"severity"`
	OccurredAt  time.Time `json:"occurred_at"`
}
