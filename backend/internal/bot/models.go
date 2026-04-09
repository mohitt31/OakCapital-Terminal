package bot

// IncomingMessage is a union struct for all messages the engine can send.
// We unmarshal into this and dispatch on Type.
type IncomingMessage struct {
	Type           string  `json:"type"`
	ClientID       string  `json:"client_id,omitempty"`
	Symbol         string  `json:"symbol,omitempty"`
	OrderID        string  `json:"order_id,omitempty"`
	Side           string  `json:"side,omitempty"`
	Status         string  `json:"status,omitempty"`
	Price          float64 `json:"price,omitempty"`
	Quantity       float64 `json:"quantity,omitempty"`
	RemainingQty   float64 `json:"remaining_qty,omitempty"`
	BestBid        float64 `json:"best_bid,omitempty"`
	BestAsk        float64 `json:"best_ask,omitempty"`
	LastTradePrice float64 `json:"last_trade_price,omitempty"`
	Volume         float64 `json:"volume,omitempty"`
	Timestamp      int64   `json:"timestamp,omitempty"`
}

// OutgoingOrder is what we send to the engine (limit, market, or cancel).
type OutgoingOrder struct {
	Type     string  `json:"type"`
	ClientID string  `json:"client_id"`
	OrderID  string  `json:"order_id"`
	Symbol   string  `json:"symbol,omitempty"`
	Side     string  `json:"side,omitempty"`
	Price    float64 `json:"price,omitempty"`
	Quantity float64 `json:"quantity,omitempty"`
}
