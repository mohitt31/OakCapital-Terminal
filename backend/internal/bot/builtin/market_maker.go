package builtin

import (
	"fmt"
	"log"
	"math"
	"sync"

	"synthbull/internal/bot"
)

// activeOrder tracks a live order we've placed.
type activeOrder struct {
	RemainingQty float64
	Status       string
}

// symbolState holds all per-symbol state.
type symbolState struct {
	portfolio    *bot.Portfolio
	activeOrders map[string]*activeOrder
	iteration    int
}

// MarketMakerStrategy implements bot.Strategy.
// It places limit bids slightly below mid-price and limit asks slightly above,
// profiting from the spread while managing inventory risk.
type MarketMakerStrategy struct {
	cfg     bot.BotConfig
	symbols map[string]*symbolState
	mu      sync.RWMutex
}

// NewMarketMakerStrategy creates a new MarketMakerStrategy with the given config.
func NewMarketMakerStrategy(cfg bot.BotConfig) (bot.Strategy, error) {
	return &MarketMakerStrategy{
		cfg:     cfg,
		symbols: make(map[string]*symbolState),
	}, nil
}

func (s *MarketMakerStrategy) Name() string { return "market_maker" }

func (s *MarketMakerStrategy) GetPNL() (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalPNL float64
	// This is a simplification. A real implementation would need to know the
	// current mark price for each symbol to calculate unrealized PNL.
	for _, ss := range s.symbols {
		totalPNL += ss.portfolio.RealizedPnL
	}
	return totalPNL, nil
}

// getOrCreateSymbol lazily initializes state for a new symbol.
func (s *MarketMakerStrategy) getOrCreateSymbolLocked(symbol string) *symbolState {
	if ss, ok := s.symbols[symbol]; ok {
		return ss
	}
	ss := &symbolState{
		portfolio:    bot.NewPortfolio(),
		activeOrders: make(map[string]*activeOrder),
	}
	s.symbols[symbol] = ss
	log.Printf("[mm] new symbol registered: %s", symbol)
	return ss
}

// computeQuotes returns (bid, ask, size, ok). ok is false when inventory
// exceeds the risk threshold and the bot should pause quoting.
func (s *MarketMakerStrategy) computeQuotes(bestBid, bestAsk, inventory float64) (bid, ask, size float64, ok bool) {
	mid := (bestBid + bestAsk) / 2.0

	// pause if inventory risk is too high
	if math.Abs(inventory) > s.cfg.InventoryRiskThreshold {
		return 0, 0, 0, false
	}

	maxPosition := s.cfg.MaxPosition
	if maxPosition <= 0 {
		maxPosition = 1
	}

	// widen spread when carrying more inventory
	dynamicSpread := s.cfg.Spread * (1 + s.cfg.DynamicSpreadMultiplier*math.Abs(inventory)/maxPosition)

	// skew quotes away from the side we're overweight on
	skew := s.cfg.InventorySkewFactor * inventory

	bid = mid - dynamicSpread/2 - skew
	ask = mid + dynamicSpread/2 + skew
	size = s.cfg.BaseOrderSize

	return bid, ask, size, true
}

// OnMarketData processes market data and returns orders to place.
func (s *MarketMakerStrategy) OnMarketData(msg bot.IncomingMessage) []bot.OutgoingOrder {
	s.mu.Lock()
	defer s.mu.Unlock()

	symbol := msg.Symbol
	if symbol == "" {
		symbol = "UNKNOWN"
	}

	ss := s.getOrCreateSymbolLocked(symbol)
	ss.iteration++

	// figure out which sides we're allowed to trade
	canBuy := true
	canSell := true
	pos := ss.portfolio.Position
	if pos >= s.cfg.MaxPosition {
		canBuy = false
	} else if pos <= -s.cfg.MaxPosition {
		canSell = false
	}

	// if inventory risk is too high, only allow the side that reduces position
	bid, ask, size, ok := s.computeQuotes(msg.BestBid, msg.BestAsk, pos)
	if !ok {
		if pos > 0 {
			canBuy = false
		} else if pos < 0 {
			canSell = false
		} else {
			s.logStatus(symbol, msg.BestBid, msg.BestAsk)
			return nil
		}
		// recompute quotes ignoring risk check for one-sided unwind
		mid := (msg.BestBid + msg.BestAsk) / 2.0
		bid = mid - s.cfg.Spread/2
		ask = mid + s.cfg.Spread/2
		size = s.cfg.BaseOrderSize
	}

	if !canBuy && !canSell {
		s.logStatus(symbol, msg.BestBid, msg.BestAsk)
		return nil
	}

	var orders []bot.OutgoingOrder

	// cancel stale orders periodically
	if s.cfg.CancelInterval > 0 && ss.iteration%s.cfg.CancelInterval == 0 {
		orders = append(orders, s.cancelAllOrders(symbol, ss)...)
	}

	// place buy limit order
	if canBuy {
		orderID := fmt.Sprintf("%s_b_%d", symbol, ss.iteration)
		orders = append(orders, bot.OutgoingOrder{
			Type:     "limit",
			OrderID:  orderID,
			Symbol:   symbol,
			Side:     "buy",
			Price:    bid,
			Quantity: size,
		})
		ss.activeOrders[orderID] = &activeOrder{RemainingQty: size, Status: "pending"}
	}

	// place sell limit order
	if canSell {
		orderID := fmt.Sprintf("%s_s_%d", symbol, ss.iteration)
		orders = append(orders, bot.OutgoingOrder{
			Type:     "limit",
			OrderID:  orderID,
			Symbol:   symbol,
			Side:     "sell",
			Price:    ask,
			Quantity: size,
		})
		ss.activeOrders[orderID] = &activeOrder{RemainingQty: size, Status: "pending"}
	}

	s.logStatus(symbol, msg.BestBid, msg.BestAsk)
	return orders
}

// OnFill processes a fill and updates portfolio.
func (s *MarketMakerStrategy) OnFill(msg bot.IncomingMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.ClientID != s.cfg.ClientID {
		return
	}

	symbol := msg.Symbol
	if symbol == "" {
		symbol = "UNKNOWN"
	}

	ss := s.getOrCreateSymbolLocked(symbol)
	ss.portfolio.UpdateOnFill(msg.Side, msg.Price, msg.Quantity)
	log.Printf("[mm][fill] %s %s %.0f @ %.2f", symbol, msg.Side, msg.Quantity, msg.Price)

	// update remaining qty on the order
	if o, ok := ss.activeOrders[msg.OrderID]; ok {
		o.RemainingQty -= msg.Quantity
		if o.RemainingQty <= 0 {
			delete(ss.activeOrders, msg.OrderID)
		}
	}
}

// OnAck updates order status based on engine acknowledgement.
func (s *MarketMakerStrategy) OnAck(msg bot.IncomingMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.ClientID != s.cfg.ClientID {
		return
	}

	symbol := msg.Symbol
	if symbol == "" {
		symbol = "UNKNOWN"
	}

	ss := s.getOrCreateSymbolLocked(symbol)
	if o, ok := ss.activeOrders[msg.OrderID]; ok {
		o.Status = msg.Status
		if msg.Status == "rejected" || msg.Status == "cancelled" {
			delete(ss.activeOrders, msg.OrderID)
		}
	}
}

// cancelAllOrders returns cancel orders for all active orders on a symbol.
func (s *MarketMakerStrategy) cancelAllOrders(symbol string, ss *symbolState) []bot.OutgoingOrder {
	var orders []bot.OutgoingOrder
	for orderID, o := range ss.activeOrders {
		if o.RemainingQty > 0 {
			orders = append(orders, bot.OutgoingOrder{
				Type:    "cancel",
				OrderID: orderID,
				Symbol:  symbol,
			})
		}
	}
	return orders
}

// logStatus prints a one-line status update for a symbol.
func (s *MarketMakerStrategy) logStatus(symbol string, bestBid, bestAsk float64) {
	ss := s.symbols[symbol]
	mid := (bestBid + bestAsk) / 2.0
	pnl := ss.portfolio.UnrealizedPnL(mid)
	log.Printf("[mm][%s] mid=%.2f pos=%.0f pnl=%.2f", symbol, mid, ss.portfolio.Position, pnl)
}

// GetSymbolState exposes symbol state for testing.
func (s *MarketMakerStrategy) GetSymbolState(symbol string) (*bot.Portfolio, map[string]*activeOrder) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if ss, ok := s.symbols[symbol]; ok {
		portfolioCopy := &bot.Portfolio{
			Position:    ss.portfolio.Position,
			Cash:        ss.portfolio.Cash,
			AvgPrice:    ss.portfolio.AvgPrice,
			RealizedPnL: ss.portfolio.RealizedPnL,
		}
		ordersCopy := make(map[string]*activeOrder, len(ss.activeOrders))
		for orderID, order := range ss.activeOrders {
			ordersCopy[orderID] = &activeOrder{
				RemainingQty: order.RemainingQty,
				Status:       order.Status,
			}
		}
		return portfolioCopy, ordersCopy
	}
	return nil, nil
}

// SetSymbolState allows tests to inject state.
func (s *MarketMakerStrategy) SetSymbolState(symbol string, p *bot.Portfolio) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.symbols[symbol] = &symbolState{
		portfolio: &bot.Portfolio{
			Position:    p.Position,
			Cash:        p.Cash,
			AvgPrice:    p.AvgPrice,
			RealizedPnL: p.RealizedPnL,
		},
		activeOrders: make(map[string]*activeOrder),
	}
}
