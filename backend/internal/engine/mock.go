//go:build !cgo

package engine

import (
	"sort"
	"sync"
	"time"
)

// Handle is a mock engine handle for when CGO is disabled.
type Handle struct {
	mu        sync.Mutex
	lastPrice int
	orders    map[int]*mockOrder
}

type mockOrder struct {
	OrderID int
	Side    int
	Qty     int
	Price   int
}

// New creates a mock matching engine instance.
func New() *Handle {
	return &Handle{
		lastPrice: 100,
		orders:    make(map[int]*mockOrder),
	}
}

// Close is a no-op for the mock engine.
func (e *Handle) Close() {}

func (e *Handle) AddLimit(orderID, side, qty, limitPrice int) (Status, OrderResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if qty <= 0 || limitPrice <= 0 {
		return StatusInvalidArg, OrderResult{}
	}
	e.orders[orderID] = &mockOrder{
		OrderID: orderID,
		Side:    side,
		Qty:     qty,
		Price:   limitPrice,
	}
	if e.lastPrice == 0 {
		e.lastPrice = limitPrice
	}
	return StatusOK, OrderResult{}
}

func (e *Handle) Market(orderID, side, qty int) (Status, OrderResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if qty <= 0 {
		return StatusInvalidArg, OrderResult{}
	}

	opposite := SideSell
	if side == SideSell {
		opposite = SideBuy
	}
	levels := e.collectSideLocked(opposite)
	if len(levels) == 0 {
		return StatusNotFound, OrderResult{}
	}
	// Buy consumes lowest asks first; sell consumes highest bids first.
	if side == SideBuy {
		sort.Slice(levels, func(i, j int) bool { return levels[i].Price < levels[j].Price })
	} else {
		sort.Slice(levels, func(i, j int) bool { return levels[i].Price > levels[j].Price })
	}

	remaining := qty
	trades := make([]Trade, 0, 4)
	for _, maker := range levels {
		if remaining <= 0 {
			break
		}
		if maker.Qty <= 0 {
			continue
		}
		fill := maker.Qty
		if fill > remaining {
			fill = remaining
		}
		maker.Qty -= fill
		remaining -= fill
		e.lastPrice = maker.Price
		trades = append(trades, Trade{
			Price:             maker.Price,
			Qty:               fill,
			MakerOrderID:      maker.OrderID,
			TakerOrderID:      orderID,
			TimestampUnixNano: time.Now().UnixNano(),
		})
		if maker.Qty <= 0 {
			delete(e.orders, maker.OrderID)
		}
	}
	if len(trades) == 0 {
		return StatusNotFound, OrderResult{}
	}
	return StatusOK, OrderResult{Trades: trades}
}

func (e *Handle) CancelLimit(orderID int) (Status, OrderResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.orders[orderID]; !ok {
		return StatusNotFound, OrderResult{}
	}
	delete(e.orders, orderID)
	return StatusOK, OrderResult{}
}

func (e *Handle) ModifyLimit(orderID, qty, limitPrice int) (Status, OrderResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	o, ok := e.orders[orderID]
	if !ok {
		return StatusNotFound, OrderResult{}
	}
	if qty <= 0 || limitPrice <= 0 {
		return StatusInvalidArg, OrderResult{}
	}
	o.Qty = qty
	o.Price = limitPrice
	return StatusOK, OrderResult{}
}

func (e *Handle) AddStop(orderID, side, qty, stopPrice int) (Status, OrderResult) {
	return StatusOK, OrderResult{}
}

func (e *Handle) CancelStop(orderID int) (Status, OrderResult) {
	return StatusOK, OrderResult{}
}

func (e *Handle) ModifyStop(orderID, qty, stopPrice int) (Status, OrderResult) {
	return StatusOK, OrderResult{}
}

func (e *Handle) AddStopLimit(orderID, side, qty, limitPrice, stopPrice int) (Status, OrderResult) {
	return StatusOK, OrderResult{}
}

func (e *Handle) CancelStopLimit(orderID int) (Status, OrderResult) {
	return StatusOK, OrderResult{}
}

func (e *Handle) ModifyStopLimit(orderID, qty, limitPrice, stopPrice int) (Status, OrderResult) {
	return StatusOK, OrderResult{}
}

func (e *Handle) BestBid() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	best := 0
	for _, o := range e.orders {
		if o.Side != SideBuy || o.Qty <= 0 {
			continue
		}
		if o.Price > best {
			best = o.Price
		}
	}
	return best
}

func (e *Handle) BestAsk() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	best := 0
	for _, o := range e.orders {
		if o.Side != SideSell || o.Qty <= 0 {
			continue
		}
		if best == 0 || o.Price < best {
			best = o.Price
		}
	}
	return best
}

func (e *Handle) LastExecutedCount() int {
	return 0
}

func (e *Handle) LastExecutedPrice() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastPrice
}

func (e *Handle) GetDepth() Depth {
	e.mu.Lock()
	defer e.mu.Unlock()
	bidAgg := map[int]int{}
	askAgg := map[int]int{}
	for _, o := range e.orders {
		if o.Qty <= 0 {
			continue
		}
		if o.Side == SideBuy {
			bidAgg[o.Price] += o.Qty
		} else {
			askAgg[o.Price] += o.Qty
		}
	}
	bids := make([]PriceLevel, 0, len(bidAgg))
	for p, q := range bidAgg {
		bids = append(bids, PriceLevel{Price: p, Volume: q})
	}
	sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })
	asks := make([]PriceLevel, 0, len(askAgg))
	for p, q := range askAgg {
		asks = append(asks, PriceLevel{Price: p, Volume: q})
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })

	bestBid := 0
	if len(bids) > 0 {
		bestBid = bids[0].Price
	}
	bestAsk := 0
	if len(asks) > 0 {
		bestAsk = asks[0].Price
	}
	return Depth{
		BestBid:   bestBid,
		BestAsk:   bestAsk,
		LastPrice: e.lastPrice,
		Bids:      bids,
		Asks:      asks,
	}
}

func (e *Handle) collectSideLocked(side int) []*mockOrder {
	out := make([]*mockOrder, 0, len(e.orders))
	for _, o := range e.orders {
		if o.Side == side && o.Qty > 0 {
			out = append(out, o)
		}
	}
	return out
}
