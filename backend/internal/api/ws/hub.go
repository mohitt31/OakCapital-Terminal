package ws

import (
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"synthbull/internal/auth"
	"synthbull/internal/engine"
	"synthbull/internal/eventbus"

	"github.com/gorilla/websocket"
)

// EventType defines the types of events that can be broadcast.
type EventType string

const (
	EventTrade     EventType = "trade"
	EventOrderbook EventType = "orderbook"
	EventCandle    EventType = "candle"
	EventTicker    EventType = "ticker"
	EventDepth     EventType = "depth"
	EventPortfolio EventType = "portfolio"
	EventBotStatus EventType = "bot_status"
)

// Message is the JSON envelope sent to WebSocket clients.
type Message struct {
	Type      EventType   `json:"type"`
	Symbol    string      `json:"symbol"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// TradeData represents a trade event payload.
type TradeData struct {
	Price        int   `json:"price"`
	Quantity     int   `json:"quantity"`
	MakerOrderID int   `json:"maker_order_id"`
	TakerOrderID int   `json:"taker_order_id"`
	Side         int   `json:"side"` // 0=sell, 1=buy (taker side)
	Timestamp    int64 `json:"timestamp"`
}

// TickerData represents current market summary for a symbol.
type TickerData struct {
	BestBid   int   `json:"best_bid"`
	BestAsk   int   `json:"best_ask"`
	LastPrice int   `json:"last_price"`
	Timestamp int64 `json:"timestamp"`
}

// CandleData represents OHLCV candlestick data.
type CandleData struct {
	Open      int   `json:"open"`
	High      int   `json:"high"`
	Low       int   `json:"low"`
	Close     int   `json:"close"`
	Volume    int   `json:"volume"`
	Timestamp int64 `json:"timestamp"`
	Interval  int   `json:"interval"` // interval in seconds (1 or 5)
}

// PortfolioData represents a user's portfolio state.
// This is a simplified version of portfolio.Portfolio for WS.
type PortfolioData struct {
	Cash         float64 `json:"cash"`
	Holdings     float64 `json:"holdings"`
	TotalValue   float64 `json:"total_value"`
	PNL          float64 `json:"pnl"`
	BlockedCash  float64 `json:"blocked_cash"`
	AvailableBuy float64 `json:"available_buy"`
}

// BotStatusData represents the state of a trading bot.
type BotStatusData struct {
	BotID     string  `json:"bot_id"`
	Status    string  `json:"status"`
	Message   string  `json:"message"`
	PNL       float64 `json:"pnl"`
	Timestamp int64   `json:"timestamp"`
}

// Client represents a single WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string // Authenticated user ID

	// Subscriptions: which symbols and event types this client wants
	mu            sync.RWMutex
	subscriptions map[string]map[EventType]bool // symbol -> event types
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// User ID to client mapping for targeted messaging
	userClients map[string]map[*Client]bool

	// Inbound messages from clients (subscribe/unsubscribe)
	register   chan *Client
	unregister chan *Client

	// Broadcast channel for point-in-time events (trades, candles)
	broadcast chan *Message

	// Protects clients map
	mu sync.RWMutex

	// Stop signal
	stopCh chan struct{}

	// CandleBuilder aggregates trades and GBM ticks into OHLCV candles (1s and 5s).
	candleBuilder *eventbus.CandleBuilder

	// authSvc for late-authentication of guest clients
	authSvc *auth.Service

	// Throttled coalescing: high-frequency depth/ticker updates are batched
	// and flushed at a fixed interval instead of sending every single update.
	throttleMu   sync.Mutex
	latestDepth  map[string]*Message
	latestTicker map[string]*Message

	// Drop tracking (only accessed from the Run goroutine)
	dropCount  int
	flushCount int
}

// NewHub creates a new Hub instance.
func NewHub(authSvc *auth.Service) *Hub {
	cb, err := eventbus.NewCandleBuilder([]string{"1s"})
	if err != nil {
		log.Printf("[ws] candle builder init failed: %v — candle messages disabled", err)
	}
	return &Hub{
		clients:       make(map[*Client]bool),
		userClients:   make(map[string]map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan *Message, 1000),
		stopCh:        make(chan struct{}),
		candleBuilder: cb,
		authSvc:       authSvc,
		latestDepth:   make(map[string]*Message),
		latestTicker:  make(map[string]*Message),
	}
}

// Run starts the hub's main event loop.
func (h *Hub) Run() {
	log.Println("[ws] hub started")
	flushTicker := time.NewTicker(100 * time.Millisecond)
	defer flushTicker.Stop()
	for {
		select {
		case <-h.stopCh:
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				client.conn.Close()
			}
			h.clients = make(map[*Client]bool)
			h.userClients = make(map[string]map[*Client]bool)
			h.mu.Unlock()
			log.Println("[ws] hub stopped")
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.userID != "" {
				if h.userClients[client.userID] == nil {
					h.userClients[client.userID] = make(map[*Client]bool)
				}
				h.userClients[client.userID][client] = true
			}
			h.mu.Unlock()
			log.Printf("[ws] client registered (total=%d, users=%d)", len(h.clients), len(h.userClients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if client.userID != "" && h.userClients[client.userID] != nil {
					delete(h.userClients[client.userID], client)
					if len(h.userClients[client.userID]) == 0 {
						delete(h.userClients, client.userID)
					}
				}
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[ws] client unregistered (total=%d, users=%d)", len(h.clients), len(h.userClients))

		case msg := <-h.broadcast:
			h.broadcastToSubscribers(msg)

		case <-flushTicker.C:
			h.flushThrottled()
		}
	}
}

// UpdateClientID safely upgrades a client from guest to authenticated status.
func (h *Hub) UpdateClientID(client *Client, newID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If already associated with a different user, clean up old mapping
	if client.userID != "" && h.userClients[client.userID] != nil {
		delete(h.userClients[client.userID], client)
		if len(h.userClients[client.userID]) == 0 {
			delete(h.userClients, client.userID)
		}
	}

	client.userID = newID
	if newID != "" {
		if h.userClients[newID] == nil {
			h.userClients[newID] = make(map[*Client]bool)
		}
		h.userClients[newID][client] = true
	}
	log.Printf("[ws] client upgraded to user %s (total users=%d)", newID, len(h.userClients))
}

// Stop gracefully shuts down the hub.
func (h *Hub) Stop() {
	select {
	case <-h.stopCh:
		// already stopped
	default:
		close(h.stopCh)
	}
}

// flushThrottled sends the latest coalesced depth/ticker messages to subscribers.
// Called every 100ms from the Run loop to cap the update rate at ~10 Hz per symbol.
func (h *Hub) flushThrottled() {
	h.throttleMu.Lock()
	depths := h.latestDepth
	tickers := h.latestTicker
	h.latestDepth = make(map[string]*Message)
	h.latestTicker = make(map[string]*Message)
	h.throttleMu.Unlock()

	for _, msg := range tickers {
		h.broadcastToSubscribers(msg)
	}
	for _, msg := range depths {
		h.broadcastToSubscribers(msg)
	}

	h.flushCount++
	if h.flushCount >= 10 {
		if h.dropCount > 0 {
			log.Printf("[ws] dropped %d messages in last ~1s (slow clients)", h.dropCount)
		}
		h.dropCount = 0
		h.flushCount = 0
	}
}

// broadcastToSubscribers sends a message only to clients subscribed to that symbol+event.
func (h *Hub) broadcastToSubscribers(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ws] marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.isSubscribed(msg.Symbol, msg.Type) {
			select {
			case client.send <- data:
			default:
				h.dropCount++
			}
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// BroadcastTrade sends trade data to all subscribed clients and feeds the candle builder.
func (h *Hub) BroadcastTrade(symbol string, trade engine.Trade, takerSide int) {
	ts := trade.TimestampUnixNano
	if ts == 0 {
		ts = time.Now().UnixNano()
	}
	tsMillis := ts / 1_000_000

	msg := &Message{
		Type:      EventTrade,
		Symbol:    symbol,
		Timestamp: tsMillis,
		Data: TradeData{
			Price:        trade.Price,
			Quantity:     trade.Qty,
			MakerOrderID: trade.MakerOrderID,
			TakerOrderID: trade.TakerOrderID,
			Side:         takerSide,
			Timestamp:    tsMillis,
		},
	}

	select {
	case h.broadcast <- msg:
	default:
		log.Printf("[ws] broadcast channel full, dropping trade")
	}

	// Feed trade into the candle builder and broadcast resulting candle events.
	if h.candleBuilder != nil {
		events := h.candleBuilder.OnTrade(eventbus.TradeEvent{
			Symbol:     symbol,
			Price:      int64(trade.Price),
			Quantity:   int64(trade.Qty),
			ExecutedAt: time.Unix(0, ts).UTC(),
		})
		for _, ce := range events {
			h.BroadcastCandle(symbol, candleEventToData(ce))
		}
	}
}

// UpdateCandlePrice feeds a GBM price tick (no volume) into the candle builder.
// Call on every GBM tick so candles update smoothly between actual trades.
// priceINR is the raw GBM price in the same units as Config.InitialPrice (e.g. INR floats).
func (h *Hub) UpdateCandlePrice(symbol string, priceINR float64, tsMillis int64) {
	if h.candleBuilder == nil {
		return
	}
	priceCents := int64(math.Round(priceINR * 100))
	ts := time.Unix(0, tsMillis*int64(time.Millisecond)).UTC()
	events := h.candleBuilder.OnGBMTick(eventbus.GBMTickEvent{
		Symbol:    symbol,
		BasePrice: priceCents,
		Timestamp: ts,
	})
	for _, ce := range events {
		h.BroadcastCandle(symbol, candleEventToData(ce))
	}
}

// candleEventToData converts an eventbus.CandleEvent to the ws.CandleData wire format.
func candleEventToData(ce eventbus.CandleEvent) CandleData {
	intervalSecs := 1
	if ce.Interval == "5s" {
		intervalSecs = 5
	}
	return CandleData{
		Open:      int(ce.Open),
		High:      int(ce.High),
		Low:       int(ce.Low),
		Close:     int(ce.Close),
		Volume:    int(ce.Volume),
		Timestamp: ce.Timestamp.UnixMilli(),
		Interval:  intervalSecs,
	}
}

// BroadcastTrades sends multiple trades at once.
func (h *Hub) BroadcastTrades(symbol string, trades []engine.Trade, takerSide int) {
	for _, trade := range trades {
		h.BroadcastTrade(symbol, trade, takerSide)
	}
}

// BroadcastTicker sends ticker (best bid/ask, last price) updates.
// Coalesced per-symbol and flushed at ~10 Hz by flushThrottled.
func (h *Hub) BroadcastTicker(symbol string, bestBid, bestAsk, lastPrice int) {
	msg := &Message{
		Type:      EventTicker,
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Data: TickerData{
			BestBid:   bestBid,
			BestAsk:   bestAsk,
			LastPrice: lastPrice,
			Timestamp: time.Now().UnixMilli(),
		},
	}

	h.throttleMu.Lock()
	h.latestTicker[symbol] = msg
	h.throttleMu.Unlock()
}

// BroadcastDepth sends order book depth snapshot.
// Coalesced per-symbol and flushed at ~10 Hz by flushThrottled.
func (h *Hub) BroadcastDepth(symbol string, depth engine.Depth) {
	msg := &Message{
		Type:      EventDepth,
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Data:      depth,
	}

	h.throttleMu.Lock()
	h.latestDepth[symbol] = msg
	h.throttleMu.Unlock()
}

// BroadcastCandle sends OHLCV candle data.
func (h *Hub) BroadcastCandle(symbol string, candle CandleData) {
	msg := &Message{
		Type:      EventCandle,
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Data:      candle,
	}

	select {
	case h.broadcast <- msg:
	default:
		log.Printf("[ws] broadcast channel full, dropping candle")
	}
}

// BroadcastToUser sends a message to all connections for a specific user ID.
func (h *Hub) BroadcastToUser(userID string, msg interface{}) {
	if userID == "" {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ws] marshal error for user %s: %v", userID, err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if userConnections, ok := h.userClients[userID]; ok {
		for client := range userConnections {
			select {
			case client.send <- data:
			default:
				h.dropCount++
			}
		}
	}
}

// isSubscribed checks if this client is subscribed to a symbol+event combination.
func (c *Client) isSubscribed(symbol string, eventType EventType) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check symbol-specific subscription
	if events, ok := c.subscriptions[symbol]; ok {
		if events[eventType] {
			return true
		}
	}

	// Check wildcard subscription (subscribe to all symbols)
	if events, ok := c.subscriptions["*"]; ok {
		if events[eventType] {
			return true
		}
	}

	return false
}

// Subscribe adds a subscription for a symbol+event combination.
func (c *Client) Subscribe(symbol string, eventType EventType) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subscriptions == nil {
		c.subscriptions = make(map[string]map[EventType]bool)
	}
	if c.subscriptions[symbol] == nil {
		c.subscriptions[symbol] = make(map[EventType]bool)
	}
	c.subscriptions[symbol][eventType] = true
	log.Printf("[ws] client %s subscribed to %s:%s", c.userID, symbol, eventType)
}

// Unsubscribe removes a subscription for a symbol+event combination.
func (c *Client) Unsubscribe(symbol string, eventType EventType) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if events, ok := c.subscriptions[symbol]; ok {
		delete(events, eventType)
		if len(events) == 0 {
			delete(c.subscriptions, symbol)
		}
	}
	log.Printf("[ws] client %s unsubscribed from %s:%s", c.userID, symbol, eventType)
}

// SubscribeAll subscribes to all event types for a symbol.
func (c *Client) SubscribeAll(symbol string) {
	c.Subscribe(symbol, EventTrade)
	c.Subscribe(symbol, EventTicker)
	c.Subscribe(symbol, EventDepth)
	c.Subscribe(symbol, EventCandle)
}

// UnsubscribeAll removes all subscriptions for a symbol.
func (c *Client) UnsubscribeAll(symbol string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscriptions, symbol)
	log.Printf("[ws] client %s unsubscribed from all %s events", c.userID, symbol)
}

// GetSubscriptions returns all current subscriptions.
func (c *Client) GetSubscriptions() map[string][]EventType {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]EventType)
	for symbol, events := range c.subscriptions {
		for event := range events {
			result[symbol] = append(result[symbol], event)
		}
	}
	return result
}
