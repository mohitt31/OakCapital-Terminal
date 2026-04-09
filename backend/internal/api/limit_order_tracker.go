package api

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"synthbull/internal/engine"
	"synthbull/internal/eventbus"
	"synthbull/internal/portfolio"
	"synthbull/pkg/models"
)

type trackedOrderKey struct {
	symbol  string
	orderID int64
}

type trackedLimitOrder struct {
	UserID             string
	Symbol             string
	Side               models.Side
	Quantity           int64
	LimitPriceCents    int64
	FilledQty          int64
	TotalNotionalCents int64
	RemainingReserve   float64
}

type TrackedLimitOrderSnapshot struct {
	UserID           string
	Symbol           string
	Side             models.Side
	Quantity         int64
	FilledQty        int64
	RemainingReserve float64
}

type orderFillAction struct {
	OrderID        int64
	UserID         string
	Symbol         string
	Side           models.Side
	FillQty        float64
	FillPrice      float64
	OrderQty       int64
	FilledQty      int64
	RemainingQty   int64
	AvgPriceCents  int64
	Status         models.OrderStatus
	ReleaseReserve float64
}

// LimitOrderTracker keeps user-owned limit orders in memory so delayed fills can
// be reconciled when market simulation later matches resting orders.
type LimitOrderTracker struct {
	mu     sync.Mutex
	orders map[trackedOrderKey]*trackedLimitOrder
}

func NewLimitOrderTracker() *LimitOrderTracker {
	return &LimitOrderTracker{
		orders: make(map[trackedOrderKey]*trackedLimitOrder),
	}
}

func normalizeTrackedSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func (t *LimitOrderTracker) orderKey(symbol string, orderID int64) trackedOrderKey {
	return trackedOrderKey{symbol: normalizeTrackedSymbol(symbol), orderID: orderID}
}

func (t *LimitOrderTracker) Track(userID, symbol string, side models.Side, orderID, quantity, limitPriceCents int64) {
	if orderID <= 0 || quantity <= 0 {
		return
	}

	reserve := 0.0
	if side == models.SideBuy {
		reserve = float64(limitPriceCents*quantity) / 100.0
	}

	t.mu.Lock()
	t.orders[t.orderKey(symbol, orderID)] = &trackedLimitOrder{
		UserID:           userID,
		Symbol:           normalizeTrackedSymbol(symbol),
		Side:             side,
		Quantity:         quantity,
		LimitPriceCents:  limitPriceCents,
		RemainingReserve: reserve,
	}
	t.mu.Unlock()
}

// RecordPlacementTrades updates tracker state for fills returned immediately
// from PlaceLimitOrder (the HTTP handler already applies those fills).
func (t *LimitOrderTracker) RecordPlacementTrades(symbol string, orderID int64, trades []engine.Trade) (releaseReserve float64, completed bool) {
	if len(trades) == 0 {
		return 0, false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	key := t.orderKey(symbol, orderID)
	order, ok := t.orders[key]
	if !ok {
		return 0, false
	}

	for _, trade := range trades {
		if int64(trade.MakerOrderID) != orderID && int64(trade.TakerOrderID) != orderID {
			continue
		}
		_, release, done, changed := applyTradeToOrder(order, int64(trade.Price), int64(trade.Qty))
		if !changed {
			continue
		}
		releaseReserve += release
		if done {
			delete(t.orders, key)
			completed = true
			break
		}
	}

	return releaseReserve, completed
}

func (t *LimitOrderTracker) Remove(symbol string, orderID int64) (TrackedLimitOrderSnapshot, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := t.orderKey(symbol, orderID)
	order, ok := t.orders[key]
	if !ok {
		return TrackedLimitOrderSnapshot{}, false
	}

	delete(t.orders, key)
	return TrackedLimitOrderSnapshot{
		UserID:           order.UserID,
		Symbol:           order.Symbol,
		Side:             order.Side,
		Quantity:         order.Quantity,
		FilledQty:        order.FilledQty,
		RemainingReserve: order.RemainingReserve,
	}, true
}

func (t *LimitOrderTracker) UpdateLimit(symbol string, orderID, quantity, limitPriceCents int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := t.orderKey(symbol, orderID)
	order, ok := t.orders[key]
	if !ok {
		return
	}
	order.Quantity = quantity
	order.LimitPriceCents = limitPriceCents
}

// ReconcileTrades applies deferred fills for tracked limit orders when the
// market generator (or any external taker flow) matches resting user orders.
func (t *LimitOrderTracker) ReconcileTrades(
	ctx context.Context,
	symbol string,
	trades []engine.Trade,
	portfolioMgr *portfolio.Manager,
	bus *eventbus.Publisher,
) {
	if len(trades) == 0 {
		return
	}

	actions := t.collectFillActions(symbol, trades)
	for _, action := range actions {
		if portfolioMgr != nil {
			if _, err := portfolioMgr.ApplyFill(
				ctx,
				action.UserID,
				action.Symbol,
				strings.ToLower(string(action.Side)),
				action.FillPrice,
				action.FillQty,
				true,
			); err != nil {
				log.Printf(
					"[order-tracker] ApplyFill failed user=%s symbol=%s order=%d qty=%.4f price=%.2f: %v",
					action.UserID,
					action.Symbol,
					action.OrderID,
					action.FillQty,
					action.FillPrice,
					err,
				)
			}

			if action.ReleaseReserve > 0 {
				portfolioMgr.ReleaseCashAmount(ctx, action.UserID, action.ReleaseReserve)
			}
		}

		if bus != nil {
			_ = bus.PublishOrderUpdate(context.Background(), eventbus.OrderUpdateEvent{
				OrderID:      action.OrderID,
				UserID:       action.UserID,
				Symbol:       action.Symbol,
				Side:         action.Side,
				Type:         models.TypeLimit,
				Status:       action.Status,
				Quantity:     action.OrderQty,
				FilledQty:    action.FilledQty,
				RemainingQty: action.RemainingQty,
				AvgPrice:     action.AvgPriceCents,
				UpdatedAt:    time.Now().UTC(),
			})
		}
	}
}

func (t *LimitOrderTracker) collectFillActions(symbol string, trades []engine.Trade) []orderFillAction {
	t.mu.Lock()
	defer t.mu.Unlock()

	actions := make([]orderFillAction, 0, len(trades)*2)
	for _, trade := range trades {
		if action, ok := t.applyTradeLocked(symbol, int64(trade.MakerOrderID), int64(trade.Price), int64(trade.Qty)); ok {
			actions = append(actions, action)
		}
		if trade.TakerOrderID != trade.MakerOrderID {
			if action, ok := t.applyTradeLocked(symbol, int64(trade.TakerOrderID), int64(trade.Price), int64(trade.Qty)); ok {
				actions = append(actions, action)
			}
		}
	}
	return actions
}

func (t *LimitOrderTracker) applyTradeLocked(symbol string, orderID, priceCents, qty int64) (orderFillAction, bool) {
	if orderID <= 0 || qty <= 0 || priceCents <= 0 {
		return orderFillAction{}, false
	}

	key := t.orderKey(symbol, orderID)
	order, ok := t.orders[key]
	if !ok {
		return orderFillAction{}, false
	}

	appliedQty, releaseReserve, completed, changed := applyTradeToOrder(order, priceCents, qty)
	if !changed {
		return orderFillAction{}, false
	}

	if completed {
		delete(t.orders, key)
	}

	remainingQty := order.Quantity - order.FilledQty
	if remainingQty < 0 {
		remainingQty = 0
	}

	status := models.StatusPartiallyFilled
	if completed {
		status = models.StatusFilled
	}

	avgPriceCents := int64(0)
	if order.FilledQty > 0 {
		avgPriceCents = order.TotalNotionalCents / order.FilledQty
	}

	return orderFillAction{
		OrderID:        orderID,
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		Side:           order.Side,
		FillQty:        float64(appliedQty),
		FillPrice:      float64(priceCents) / 100.0,
		OrderQty:       order.Quantity,
		FilledQty:      order.FilledQty,
		RemainingQty:   remainingQty,
		AvgPriceCents:  avgPriceCents,
		Status:         status,
		ReleaseReserve: releaseReserve,
	}, true
}

func applyTradeToOrder(order *trackedLimitOrder, priceCents, qty int64) (appliedQty int64, releaseReserve float64, completed bool, changed bool) {
	remaining := order.Quantity - order.FilledQty
	if remaining <= 0 || qty <= 0 {
		return 0, 0, false, false
	}

	fillQty := minInt64(qty, remaining)
	if fillQty <= 0 {
		return 0, 0, false, false
	}

	order.FilledQty += fillQty
	order.TotalNotionalCents += fillQty * priceCents

	if order.Side == models.SideBuy {
		used := float64(fillQty*priceCents) / 100.0
		order.RemainingReserve -= used
		if order.RemainingReserve < 0 {
			order.RemainingReserve = 0
		}
	}

	if order.FilledQty >= order.Quantity {
		releaseReserve = order.RemainingReserve
		order.RemainingReserve = 0
		return fillQty, releaseReserve, true, true
	}

	return fillQty, 0, false, true
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
