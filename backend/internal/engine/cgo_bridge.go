//go:build cgo

// CGO bridge — calls extern "C" functions from the compiled C++ matching engine
// shared library (.so/.dylib). This is the Go side of the Go↔C++  boundary.
// All order operations (limit, market, stop, cancel, modify) and book queries
// (depth, best bid/ask) go through here.
package engine

/*
#cgo CFLAGS: -I${SRCDIR}/../../Matching-Engine/include
#cgo LDFLAGS: -L${SRCDIR}/../../lib -lmatching_engine_c_api -Wl,-rpath,${SRCDIR}/../../lib

#include "engine_c_api.h"
*/
import "C"

import (
	"time"
)

// Handle wraps the opaque C engine pointer.
type Handle struct {
	h C.engine_handle_t
}

// New creates a new matching engine instance (one per symbol/book).
func New() *Handle {
	return &Handle{h: C.engine_create()}
}

// Close destroys the engine and frees C++ memory.
func (e *Handle) Close() {
	if e == nil || e.h == nil {
		return
	}
	C.engine_destroy(e.h)
	e.h = nil
}

func parseResult(raw C.engine_order_result_t) OrderResult {
	changeCount := int(raw.change_count)
	changes := make([]LevelChange, changeCount)
	for i := 0; i < changeCount; i++ {
		changes[i] = LevelChange{
			Price:  int(raw.changes[i].price),
			Volume: int(raw.changes[i].volume),
			Side:   int(raw.changes[i].side),
		}
	}

	tradeCount := int(raw.trade_count)
	trades := make([]Trade, tradeCount)
	for i := 0; i < tradeCount; i++ {
		trades[i] = Trade{
			Price:        int(raw.trades[i].price),
			Qty:          int(raw.trades[i].qty),
			MakerOrderID: int(raw.trades[i].maker_order_id),
			TakerOrderID: int(raw.trades[i].taker_order_id),
		}
	}

	return OrderResult{
		Changes: changes,
		Trades:  trades,
	}
}

func stampTrades(result OrderResult, ts int64) OrderResult {
	for i := range result.Trades {
		if result.Trades[i].TimestampUnixNano == 0 {
			result.Trades[i].TimestampUnixNano = ts
		}
	}
	return result
}

// --- Limit Orders ---

func (e *Handle) AddLimit(orderID, side, qty, limitPrice int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_add_limit(e.h, C.int(orderID), C.int(side), C.int(qty), C.int(limitPrice), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

func (e *Handle) Market(orderID, side, qty int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_market(e.h, C.int(orderID), C.int(side), C.int(qty), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

func (e *Handle) CancelLimit(orderID int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_cancel_limit(e.h, C.int(orderID), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

func (e *Handle) ModifyLimit(orderID, qty, limitPrice int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_modify_limit(e.h, C.int(orderID), C.int(qty), C.int(limitPrice), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

// --- Stop Orders ---

func (e *Handle) AddStop(orderID, side, qty, stopPrice int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_add_stop(e.h, C.int(orderID), C.int(side), C.int(qty), C.int(stopPrice), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

func (e *Handle) CancelStop(orderID int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_cancel_stop(e.h, C.int(orderID), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

func (e *Handle) ModifyStop(orderID, qty, stopPrice int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_modify_stop(e.h, C.int(orderID), C.int(qty), C.int(stopPrice), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

// --- Stop-Limit Orders ---

func (e *Handle) AddStopLimit(orderID, side, qty, limitPrice, stopPrice int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_add_stop_limit(e.h, C.int(orderID), C.int(side), C.int(qty), C.int(limitPrice), C.int(stopPrice), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

func (e *Handle) CancelStopLimit(orderID int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_cancel_stop_limit(e.h, C.int(orderID), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

func (e *Handle) ModifyStopLimit(orderID, qty, limitPrice, stopPrice int) (Status, OrderResult) {
	var raw C.engine_order_result_t
	s := Status(C.engine_modify_stop_limit(e.h, C.int(orderID), C.int(qty), C.int(limitPrice), C.int(stopPrice), &raw))
	return s, stampTrades(parseResult(raw), time.Now().UTC().UnixNano())
}

// --- Queries ---

func (e *Handle) BestBid() int {
	return int(C.engine_best_bid(e.h))
}

func (e *Handle) BestAsk() int {
	return int(C.engine_best_ask(e.h))
}

func (e *Handle) LastExecutedCount() int {
	return int(C.engine_last_executed_count(e.h))
}

func (e *Handle) LastExecutedPrice() int {
	return int(C.engine_last_executed_price(e.h))
}

func (e *Handle) GetDepth() Depth {
	var raw C.engine_depth_t
	C.engine_get_depth(e.h, &raw)

	bidCount := int(raw.bid_count)
	bids := make([]PriceLevel, bidCount)
	for i := 0; i < bidCount; i++ {
		bids[i] = PriceLevel{Price: int(raw.bids[i].price), Volume: int(raw.bids[i].volume)}
	}

	askCount := int(raw.ask_count)
	asks := make([]PriceLevel, askCount)
	for i := 0; i < askCount; i++ {
		asks[i] = PriceLevel{Price: int(raw.asks[i].price), Volume: int(raw.asks[i].volume)}
	}

	return Depth{
		BestBid:   int(raw.best_bid),
		BestAsk:   int(raw.best_ask),
		LastPrice: int(raw.last_price),
		Bids:      bids,
		Asks:      asks,
	}
}
