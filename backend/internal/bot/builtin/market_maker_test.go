package builtin

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"synthbull/internal/bot"

	"github.com/gorilla/websocket"
)

// mockExchange simulates a matching engine for testing the bot.
type mockExchange struct {
	mu       sync.Mutex
	writeMu  sync.Mutex
	conn     *websocket.Conn
	received []bot.OutgoingOrder // orders received from bot
	fills    int
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// startMockServer creates a test websocket server that acts like the exchange.
// It sends market data for the given symbols and responds to limit orders with
// acks and partial fills so we can exercise the full bot flow.
func startMockServer(t *testing.T) (*httptest.Server, *mockExchange) {
	me := &mockExchange{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		me.mu.Lock()
		me.conn = conn
		me.mu.Unlock()

		// read loop: collect orders from bot, send acks and fills
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var order bot.OutgoingOrder
			if err := json.Unmarshal(data, &order); err != nil {
				continue
			}

			me.mu.Lock()
			me.received = append(me.received, order)
			me.mu.Unlock()

			log.Printf("[mock] got %s order: %s %s %.2f x %.0f",
				order.Type, order.Symbol, order.Side, order.Price, order.Quantity)

			// send ack for limit orders
			if order.Type == "limit" {
				ack := bot.IncomingMessage{
					Type:         "ack",
					ClientID:     order.ClientID,
					Symbol:       order.Symbol,
					OrderID:      order.OrderID,
					Status:       "accepted",
					RemainingQty: order.Quantity,
				}
				ackData, _ := json.Marshal(ack)
				me.writeMu.Lock()
				conn.WriteMessage(websocket.TextMessage, ackData)
				me.writeMu.Unlock()

				// simulate a fill for half the order to exercise fill handling
				me.mu.Lock()
				me.fills++
				fillQty := order.Quantity / 2
				if fillQty < 1 {
					fillQty = 1
				}
				me.mu.Unlock()

				fill := bot.IncomingMessage{
					Type:         "fill",
					ClientID:     order.ClientID,
					Symbol:       order.Symbol,
					OrderID:      order.OrderID,
					Side:         order.Side,
					Price:        order.Price,
					Quantity:     fillQty,
					RemainingQty: order.Quantity - fillQty,
				}
				fillData, _ := json.Marshal(fill)
				me.writeMu.Lock()
				conn.WriteMessage(websocket.TextMessage, fillData)
				me.writeMu.Unlock()
			}

			// ack cancels
			if order.Type == "cancel" {
				ack := bot.IncomingMessage{
					Type:     "ack",
					ClientID: order.ClientID,
					Symbol:   order.Symbol,
					OrderID:  order.OrderID,
					Status:   "cancelled",
				}
				ackData, _ := json.Marshal(ack)
				me.writeMu.Lock()
				conn.WriteMessage(websocket.TextMessage, ackData)
				me.writeMu.Unlock()
			}
		}
	}))

	return server, me
}

// sendMarketData pushes a market_data message from the mock server to the bot.
func (me *mockExchange) sendMarketData(symbol string, bid, ask float64) error {
	me.mu.Lock()
	defer me.mu.Unlock()
	if me.conn == nil {
		return fmt.Errorf("no connection")
	}
	md := bot.IncomingMessage{
		Type:    "market_data",
		Symbol:  symbol,
		BestBid: bid,
		BestAsk: ask,
	}
	data, _ := json.Marshal(md)
	me.writeMu.Lock()
	defer me.writeMu.Unlock()
	return me.conn.WriteMessage(websocket.TextMessage, data)
}

// sendTrade pushes a trade broadcast from the mock server.
func (me *mockExchange) sendTrade(symbol string, price, qty float64, side string) error {
	me.mu.Lock()
	defer me.mu.Unlock()
	if me.conn == nil {
		return fmt.Errorf("no connection")
	}
	t := bot.IncomingMessage{
		Type:     "trade",
		Symbol:   symbol,
		Price:    price,
		Quantity: qty,
		Side:     side,
	}
	data, _ := json.Marshal(t)
	me.writeMu.Lock()
	defer me.writeMu.Unlock()
	return me.conn.WriteMessage(websocket.TextMessage, data)
}

// ordersForSymbol returns all non-cancel orders received for a given symbol.
func (me *mockExchange) ordersForSymbol(symbol string) []bot.OutgoingOrder {
	me.mu.Lock()
	defer me.mu.Unlock()
	var result []bot.OutgoingOrder
	for _, o := range me.received {
		if o.Symbol == symbol && o.Type != "cancel" {
			result = append(result, o)
		}
	}
	return result
}

// cancelsForSymbol returns all cancel orders received for a given symbol.
func (me *mockExchange) cancelsForSymbol(symbol string) []bot.OutgoingOrder {
	me.mu.Lock()
	defer me.mu.Unlock()
	var result []bot.OutgoingOrder
	for _, o := range me.received {
		if o.Symbol == symbol && o.Type == "cancel" {
			result = append(result, o)
		}
	}
	return result
}

// newTestBot creates a BotManager, registers market_maker, creates and starts a bot
// connected to the given mock server. Returns the manager, strategy, and cleanup func.
func newTestBot(t *testing.T, server *httptest.Server) (*bot.BotManager, *MarketMakerStrategy) {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	t.Setenv("WS_URL", wsURL)

	cfg := bot.DefaultConfig()
	mgr := bot.NewManager(nil)
	mgr.RegisterStrategy("market_maker", NewMarketMakerStrategy)

	inst, err := mgr.Create("test-user", "test-mm", "market_maker", cfg)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := mgr.Start("test-mm"); err != nil {
		t.Fatalf("start: %v", err)
	}

	strat := inst.Strategy.(*MarketMakerStrategy)
	return mgr, strat
}

// --- Tests ---

func TestBotConnectsAndPlacesOrders(t *testing.T) {
	server, me := startMockServer(t)
	defer server.Close()

	mgr, strat := newTestBot(t, server)
	defer mgr.Stop("test-mm")

	// send market data and wait for bot to react
	if err := me.sendMarketData("AAPL", 150.0, 150.5); err != nil {
		t.Fatalf("send market data: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	// verify bot placed orders
	orders := me.ordersForSymbol("AAPL")
	if len(orders) < 2 {
		t.Fatalf("expected at least 2 orders (buy+sell), got %d", len(orders))
	}

	hasBuy, hasSell := false, false
	for _, o := range orders {
		if o.Side == "buy" {
			hasBuy = true
		}
		if o.Side == "sell" {
			hasSell = true
		}
	}
	if !hasBuy || !hasSell {
		t.Fatalf("expected both buy and sell orders, got buy=%v sell=%v", hasBuy, hasSell)
	}

	// verify portfolio updated from fill
	portfolio, _ := strat.GetSymbolState("AAPL")
	if portfolio == nil {
		t.Fatal("AAPL symbol state not created")
	}
	if portfolio.Position == 0 && portfolio.RealizedPnL == 0 {
		t.Log("warning: position is 0, fills may not have been processed yet")
	}
	t.Logf("AAPL portfolio: %s", portfolio)
}

func TestMultiSymbol(t *testing.T) {
	server, me := startMockServer(t)
	defer server.Close()

	mgr, strat := newTestBot(t, server)
	defer mgr.Stop("test-mm")

	// send market data for 3 different symbols
	me.sendMarketData("AAPL", 150.0, 150.5)
	time.Sleep(100 * time.Millisecond)
	me.sendMarketData("GOOG", 2800.0, 2801.0)
	time.Sleep(100 * time.Millisecond)
	me.sendMarketData("TSLA", 250.0, 250.8)
	time.Sleep(200 * time.Millisecond)

	// all 3 symbols should have state
	for _, sym := range []string{"AAPL", "GOOG", "TSLA"} {
		p, _ := strat.GetSymbolState(sym)
		if p == nil {
			t.Errorf("symbol %s not initialized", sym)
		}
	}

	// each should have orders
	for _, sym := range []string{"AAPL", "GOOG", "TSLA"} {
		orders := me.ordersForSymbol(sym)
		if len(orders) < 2 {
			t.Errorf("symbol %s: expected >=2 orders, got %d", sym, len(orders))
		}
	}

	// log all portfolio states
	for _, sym := range []string{"AAPL", "GOOG", "TSLA"} {
		p, _ := strat.GetSymbolState(sym)
		if p != nil {
			t.Logf("%s: %s", sym, p)
		}
	}
}

func TestCancelOrders(t *testing.T) {
	server, me := startMockServer(t)
	defer server.Close()

	mgr, _ := newTestBot(t, server)
	defer mgr.Stop("test-mm")

	// send enough market data ticks to trigger cancel (CancelInterval = 5)
	for i := 0; i < 6; i++ {
		me.sendMarketData("ETH", 3000.0+float64(i), 3001.0+float64(i))
		time.Sleep(100 * time.Millisecond)
	}

	cancels := me.cancelsForSymbol("ETH")
	if len(cancels) == 0 {
		t.Error("expected cancel orders after 5 iterations, got none")
	}
	t.Logf("received %d cancel orders", len(cancels))
}

func TestInventoryRiskPause(t *testing.T) {
	server, me := startMockServer(t)
	defer server.Close()

	mgr, strat := newTestBot(t, server)
	defer mgr.Stop("test-mm")

	// wait for connection to establish
	time.Sleep(100 * time.Millisecond)

	// manually set position above risk threshold to test pause
	p := bot.NewPortfolio()
	cfg := bot.DefaultConfig()
	p.Position = cfg.InventoryRiskThreshold + 5
	strat.SetSymbolState("BTC", p)

	me.mu.Lock()
	me.received = nil // clear previous orders
	me.mu.Unlock()

	// send market data, bot should place sell to unwind but NOT buy
	me.sendMarketData("BTC", 50000.0, 50010.0)
	time.Sleep(200 * time.Millisecond)

	orders := me.ordersForSymbol("BTC")
	for _, o := range orders {
		if o.Side == "buy" {
			t.Errorf("should NOT buy when inventory risk exceeded on long side, got: %+v", o)
		}
	}
	// should still place sell orders to reduce inventory
	hasSell := false
	for _, o := range orders {
		if o.Side == "sell" {
			hasSell = true
		}
	}
	if !hasSell {
		t.Error("expected sell orders to unwind risk, got none")
	}
}

func TestPositionLimits(t *testing.T) {
	server, me := startMockServer(t)
	defer server.Close()

	mgr, strat := newTestBot(t, server)
	defer mgr.Stop("test-mm")

	// wait for connection
	time.Sleep(100 * time.Millisecond)

	// set position at max long - should only sell, not buy
	cfg := bot.DefaultConfig()
	p := bot.NewPortfolio()
	p.Position = cfg.MaxPosition
	p.AvgPrice = 100.0
	strat.SetSymbolState("SOL", p)

	me.mu.Lock()
	me.received = nil
	me.mu.Unlock()

	me.sendMarketData("SOL", 100.0, 100.5)
	time.Sleep(200 * time.Millisecond)

	orders := me.ordersForSymbol("SOL")
	for _, o := range orders {
		if o.Side == "buy" {
			t.Errorf("should NOT buy when at max long position, got buy order: %+v", o)
		}
	}

	hasSell := false
	for _, o := range orders {
		if o.Side == "sell" {
			hasSell = true
		}
	}
	if !hasSell {
		t.Error("should still place sell orders when at max long position")
	}
}

func TestPortfolioAccountingUnit(t *testing.T) {
	// unit test for portfolio math without the exchange
	p := bot.NewPortfolio()

	// buy 10 at 100
	p.UpdateOnFill("buy", 100.0, 10)
	if p.Position != 10 {
		t.Errorf("expected position 10, got %.0f", p.Position)
	}
	if p.AvgPrice != 100.0 {
		t.Errorf("expected avg price 100, got %.2f", p.AvgPrice)
	}

	// sell 5 at 110 (partial close with profit)
	p.UpdateOnFill("sell", 110.0, 5)
	if p.Position != 5 {
		t.Errorf("expected position 5, got %.0f", p.Position)
	}
	if p.RealizedPnL != 50.0 { // (110-100)*5 = 50
		t.Errorf("expected realized pnl 50, got %.2f", p.RealizedPnL)
	}

	// unrealized at mid=105
	u := p.UnrealizedPnL(105.0)
	if u != 25.0 { // (105-100)*5 = 25
		t.Errorf("expected unrealized pnl 25, got %.2f", u)
	}

	t.Logf("portfolio: %s", p)
}

func TestStrategyQuotes(t *testing.T) {
	cfg := bot.DefaultConfig()
	s_raw, _ := NewMarketMakerStrategy(cfg)
	s := s_raw.(*MarketMakerStrategy)

	md := bot.IncomingMessage{
		Type:    "market_data",
		Symbol:  "TEST",
		BestBid: 100.0,
		BestAsk: 100.5,
	}
	orders := s.OnMarketData(md)

	if len(orders) < 2 {
		t.Fatalf("expected at least 2 orders, got %d", len(orders))
	}

	mid := (100.0 + 100.5) / 2.0
	var bidOrder, askOrder bot.OutgoingOrder
	for _, o := range orders {
		if o.Side == "buy" {
			bidOrder = o
		}
		if o.Side == "sell" {
			askOrder = o
		}
	}

	if bidOrder.Price >= mid || askOrder.Price <= mid {
		t.Errorf("bid (%.4f) should be below mid (%.4f) and ask (%.4f) above", bidOrder.Price, mid, askOrder.Price)
	}
	if bidOrder.Quantity != cfg.BaseOrderSize {
		t.Errorf("expected size=%v, got %.0f", cfg.BaseOrderSize, bidOrder.Quantity)
	}
	t.Logf("bid=%.4f ask=%.4f size=%.0f spread=%.4f", bidOrder.Price, askOrder.Price, bidOrder.Quantity, askOrder.Price-bidOrder.Price)

	// skew test: long position should push bid down more
	p := bot.NewPortfolio()
	p.Position = 10
	s.SetSymbolState("TEST2", p)

	md2 := bot.IncomingMessage{
		Type:    "market_data",
		Symbol:  "TEST2",
		BestBid: 100.0,
		BestAsk: 100.5,
	}
	orders2 := s.OnMarketData(md2)

	var bid2, ask2 float64
	for _, o := range orders2 {
		if o.Side == "buy" {
			bid2 = o.Price
		}
		if o.Side == "sell" {
			ask2 = o.Price
		}
	}

	if bid2 >= bidOrder.Price {
		t.Errorf("with long inventory, bid should be lower: neutral=%.4f long=%.4f", bidOrder.Price, bid2)
	}
	if ask2 <= askOrder.Price {
		t.Errorf("with long inventory, ask should be higher: neutral=%.4f long=%.4f", askOrder.Price, ask2)
	}
	t.Logf("with +10 inventory: bid=%.4f ask=%.4f", bid2, ask2)
}

func TestTradeMessageHandling(t *testing.T) {
	server, me := startMockServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	t.Setenv("WS_URL", wsURL)

	cfg := bot.DefaultConfig()
	mgr := bot.NewManager(nil)
	mgr.RegisterStrategy("market_maker", NewMarketMakerStrategy)

	_, err := mgr.Create("test-user", "test-trade", "market_maker", cfg)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := mgr.Start("test-trade"); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer mgr.Stop("test-trade")

	// send trade message — bot should handle it without crashing
	time.Sleep(100 * time.Millisecond)
	me.sendTrade("DOGE", 0.25, 1000, "buy")
	time.Sleep(200 * time.Millisecond)

	// if we got this far, the trade message was handled without panic
	t.Log("trade message handled successfully")
}

func TestCancelIntervalZeroDoesNotPanic(t *testing.T) {
	cfg := bot.DefaultConfig()
	cfg.CancelInterval = 0

	s_raw, _ := NewMarketMakerStrategy(cfg)
	s := s_raw.(*MarketMakerStrategy)
	md := bot.IncomingMessage{
		Type:    "market_data",
		Symbol:  "SAFE",
		BestBid: 100.0,
		BestAsk: 100.5,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("OnMarketData should not panic when CancelInterval=0, recovered: %v", r)
		}
	}()

	orders := s.OnMarketData(md)
	if len(orders) == 0 {
		t.Fatal("expected strategy to still generate orders with CancelInterval=0")
	}
}

func TestMaxPositionZeroProducesFiniteQuotes(t *testing.T) {
	cfg := bot.DefaultConfig()
	cfg.MaxPosition = 0

	s_raw, _ := NewMarketMakerStrategy(cfg)
	s := s_raw.(*MarketMakerStrategy)
	p := bot.NewPortfolio()
	p.Position = 5
	s.SetSymbolState("FINITE", p)

	orders := s.OnMarketData(bot.IncomingMessage{
		Type:    "market_data",
		Symbol:  "FINITE",
		BestBid: 100.0,
		BestAsk: 101.0,
	})

	if len(orders) == 0 {
		t.Fatal("expected orders for finite quote validation")
	}

	for _, o := range orders {
		if o.Type == "limit" && (math.IsNaN(o.Price) || math.IsInf(o.Price, 0)) {
			t.Fatalf("expected finite quote price, got %.4f for order %+v", o.Price, o)
		}
	}
}
