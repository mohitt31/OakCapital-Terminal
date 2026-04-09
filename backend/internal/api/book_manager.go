package api

import (
	"sync"
	"sync/atomic"

	"synthbull/internal/engine"
	"synthbull/pkg/models"
)

// Broadcaster is the minimal interface the BookManager needs to push market data
// to WebSocket clients. The ws.Hub implements this directly.
type Broadcaster interface {
	BroadcastTrades(symbol string, trades []engine.Trade, takerSide int)
	BroadcastTicker(symbol string, bestBid, bestAsk, lastPrice int)
	BroadcastDepth(symbol string, depth engine.Depth)
}

type ManagedBook struct {
	mu          sync.Mutex
	nextOrderID atomic.Int64
	engine      *engine.Handle
	pub         Broadcaster
	symbol      string
}

func NewManagedBook(symbol string, pub Broadcaster) *ManagedBook {
	b := &ManagedBook{
		symbol: symbol,
		pub:    pub,
		engine: engine.New(),
	}
	b.nextOrderID.Store(0)
	return b
}

func (b *ManagedBook) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.engine.Close()
}

// broadcast pushes trade, ticker, and depth updates to all subscribed WS clients.
// Called after every successful engine operation. No-op when pub is nil.
func (b *ManagedBook) broadcast(res engine.OrderResult, takerSide models.Side) {
	if b.pub == nil {
		return
	}

	if len(res.Trades) > 0 {
		takerInt := engine.SideSell
		if takerSide == models.SideBuy {
			takerInt = engine.SideBuy
		}
		b.pub.BroadcastTrades(b.symbol, res.Trades, takerInt)

		depth := b.engine.GetDepth()
		b.pub.BroadcastTicker(b.symbol, depth.BestBid, depth.BestAsk, depth.LastPrice)
	}

	// Always push a depth snapshot so the order book stays current.
	depth := b.engine.GetDepth()
	b.pub.BroadcastDepth(b.symbol, depth)
}

// Side conversion helper for internal engine calls.
func (b *ManagedBook) sideToModels(side int) models.Side {
	if side == engine.SideBuy {
		return models.SideBuy
	}
	return models.SideSell
}

func (b *ManagedBook) AddLimit(orderID, side, qty, limitPrice int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	status, res := b.engine.AddLimit(orderID, side, qty, limitPrice)
	if status == engine.StatusOK {
		b.broadcast(res, b.sideToModels(side))
	}
	return status, res
}

func (b *ManagedBook) Market(orderID, side, qty int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	status, res := b.engine.Market(orderID, side, qty)
	if status == engine.StatusOK {
		b.broadcast(res, b.sideToModels(side))
	}
	return status, res
}

func (b *ManagedBook) CancelLimit(orderID int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	status, res := b.engine.CancelLimit(orderID)
	if status == engine.StatusOK {
		b.broadcast(res, "") // no taker side for cancel
	}
	return status, res
}

func (b *ManagedBook) ModifyLimit(orderID, qty, limitPrice int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	status, res := b.engine.ModifyLimit(orderID, qty, limitPrice)
	if status == engine.StatusOK {
		b.broadcast(res, "")
	}
	return status, res
}

func (b *ManagedBook) AddStop(orderID, side, qty, stopPrice int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	status, res := b.engine.AddStop(orderID, side, qty, stopPrice)
	return status, res // No broadcast for stop entry (not matched yet)
}

func (b *ManagedBook) CancelStop(orderID int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.engine.CancelStop(orderID)
}

func (b *ManagedBook) ModifyStop(orderID, qty, stopPrice int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.engine.ModifyStop(orderID, qty, stopPrice)
}

func (b *ManagedBook) AddStopLimit(orderID, side, qty, limitPrice, stopPrice int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.engine.AddStopLimit(orderID, side, qty, limitPrice, stopPrice)
}

func (b *ManagedBook) CancelStopLimit(orderID int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.engine.CancelStopLimit(orderID)
}

func (b *ManagedBook) ModifyStopLimit(orderID, qty, limitPrice, stopPrice int) (engine.Status, engine.OrderResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.engine.ModifyStopLimit(orderID, qty, limitPrice, stopPrice)
}

// Auto-ID wrappers used by the REST API.
func (b *ManagedBook) AddLimitAutoID(side, qty, limitPrice int) (int, engine.Status, engine.OrderResult) {
	id := int(b.nextOrderID.Add(1))
	status, res := b.AddLimit(id, side, qty, limitPrice)
	return id, status, res
}

func (b *ManagedBook) MarketAutoID(side, qty int) (int, engine.Status, engine.OrderResult) {
	id := int(b.nextOrderID.Add(1))
	status, res := b.Market(id, side, qty)
	return id, status, res
}

func (b *ManagedBook) AddStopAutoID(side, qty, stopPrice int) (int, engine.Status, engine.OrderResult) {
	id := int(b.nextOrderID.Add(1))
	status, res := b.AddStop(id, side, qty, stopPrice)
	return id, status, res
}

func (b *ManagedBook) AddStopLimitAutoID(side, qty, limitPrice, stopPrice int) (int, engine.Status, engine.OrderResult) {
	id := int(b.nextOrderID.Add(1))
	status, res := b.AddStopLimit(id, side, qty, limitPrice, stopPrice)
	return id, status, res
}

func (b *ManagedBook) BestBid() int           { return b.engine.BestBid() }
func (b *ManagedBook) BestAsk() int           { return b.engine.BestAsk() }
func (b *ManagedBook) LastExecutedCount() int { return b.engine.LastExecutedCount() }
func (b *ManagedBook) LastExecutedPrice() int { return b.engine.LastExecutedPrice() }
func (b *ManagedBook) GetDepth() engine.Depth { return b.engine.GetDepth() }

// GetHandle returns the underlying engine handle.
// Used by the market simulator to share a single engine with the REST API.
func (b *ManagedBook) GetHandle() *engine.Handle { return b.engine }

// GetMu returns the per-book mutex so the market generator uses the same lock
// as the REST API handlers.
func (b *ManagedBook) GetMu() *sync.Mutex { return &b.mu }

type BookManager struct {
	mu    sync.RWMutex
	books map[string]*ManagedBook
	pub   Broadcaster
}

func NewBookManager(pub Broadcaster) *BookManager {
	return &BookManager{
		books: make(map[string]*ManagedBook),
		pub:   pub,
	}
}

func (bm *BookManager) GetOrCreate(symbol string) *ManagedBook {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if book, exists := bm.books[symbol]; exists {
		return book
	}

	book := NewManagedBook(symbol, bm.pub)
	bm.books[symbol] = book
	return book
}

func (bm *BookManager) Get(symbol string) *ManagedBook {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.books[symbol]
}

func (bm *BookManager) Exists(symbol string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	_, exists := bm.books[symbol]
	return exists
}

func (bm *BookManager) ListSymbols() []string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	symbols := make([]string, 0, len(bm.books))
	for symbol := range bm.books {
		symbols = append(symbols, symbol)
	}
	return symbols
}

func (bm *BookManager) Delete(symbol string) bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if book, exists := bm.books[symbol]; exists {
		book.Close()
		delete(bm.books, symbol)
		return true
	}
	return false
}

func (bm *BookManager) Close() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for _, book := range bm.books {
		book.Close()
	}
	bm.books = make(map[string]*ManagedBook)
}
