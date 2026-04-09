package simbot

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// BotInstance — a single simulation bot that runs in its own goroutine.
//
// Flow:
// 1. Connects to the exchange WebSocket
// 2. Receives price data on each message
// 3. Runs the GraphEvaluator on each price tick
// 4. When graph evaluator produces OrderActions → sends orders to exchange
// 5. Tracks fills, position, P&L
// 6. Pushes BotStateUpdate to the manager's update channel
// ──────────────────────────────────────────────────────────────────────────────

type BotInstance struct {
	mu sync.RWMutex

	ID            string
	Symbol        string
	Mode          BotMode
	UserID        string
	StrategyLabel string
	Strategy      StrategyGraph
	Status        BotStatus
	PnL           float64
	Logs          []BotLog
	Position      float64
	AvgEntry      float64
	Realized      float64

	// Internal
	evaluator    *GraphEvaluator
	client       *ExchangeClient
	cancelFunc   context.CancelFunc
	updateCh     chan<- BotStateUpdate
	logCounter   atomic.Int64
	orderCounter atomic.Int64
	clientID     string

	// Portfolio tracking
	portfolioValue float64
	initialCapital float64
	evalInterval   time.Duration
}

// NewBotInstance creates a new bot but does not start it.
func NewBotInstance(id string, symbol string, strategy StrategyGraph, mode BotMode, userID string, strategyLabel string, evalInterval time.Duration, updateCh chan<- BotStateUpdate) *BotInstance {
	if mode == "" {
		mode = ModeSimulation
	}
	if evalInterval <= 0 {
		evalInterval = 1 * time.Second
	}
	const initialCapital = 10000.0
	return &BotInstance{
		ID:             id,
		Symbol:         symbol,
		Mode:           mode,
		UserID:         userID,
		StrategyLabel:  strategyLabel,
		Strategy:       strategy,
		Status:         StatusIdle,
		Logs:           make([]BotLog, 0, MaxLogs),
		evaluator:      NewGraphEvaluator(strategy, evalInterval),
		client:         &ExchangeClient{},
		updateCh:       updateCh,
		clientID:       fmt.Sprintf("simbot_%s", id),
		portfolioValue: initialCapital,
		initialCapital: initialCapital,
		evalInterval:   evalInterval,
	}
}

// Start launches the bot in a new goroutine.
func (b *BotInstance) Start(ctx context.Context) error {
	// Connect to exchange
	if err := b.client.Connect(); err != nil {
		b.setStatus(StatusError)
		b.addLog(fmt.Sprintf("Failed to connect: %v", err), "error")
		return fmt.Errorf("connect: %w", err)
	}
	if err := b.client.SubscribeTicker(b.Symbol); err != nil {
		b.setStatus(StatusError)
		b.addLog(fmt.Sprintf("Failed to subscribe ticker: %v", err), "error")
		return fmt.Errorf("subscribe ticker: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	b.cancelFunc = cancel

	b.setStatus(StatusRunning)
	b.addLog(fmt.Sprintf("Bot started (eval=%s)", FormatEvalInterval(b.evalInterval)), "info")

	go b.runLoop(childCtx)

	return nil
}

// Stop gracefully stops the bot.
func (b *BotInstance) Stop() {
	b.mu.Lock()
	if b.cancelFunc != nil {
		b.cancelFunc()
	}
	b.mu.Unlock()
}

// GetState returns the current bot state snapshot.
func (b *BotInstance) GetState() BotStateUpdate {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return BotStateUpdate{
		BotID:         b.ID,
		Status:        b.Status,
		PnL:           b.PnL,
		Symbol:        b.Symbol,
		Mode:          b.Mode,
		StrategyLabel: b.StrategyLabel,
		UserID:        b.UserID,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal
// ──────────────────────────────────────────────────────────────────────────────

func (b *BotInstance) runLoop(ctx context.Context) {
	defer func() {
		b.client.Close()
		b.setStatus(StatusStopped)
		b.addLog("Bot stopped", "info")
	}()

	// Read messages from exchange in a loop
	msgCh := make(chan IncomingMessage, 64)
	errCh := make(chan error, 1)

	go func() {
		for {
			msg, err := b.client.Recv()
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			select {
			case msgCh <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case err := <-errCh:
			b.setStatus(StatusError)
			b.addLog(fmt.Sprintf("Exchange error: %v", err), "error")
			return

		case msg := <-msgCh:
			b.handleMessage(msg)
		}
	}
}

func (b *BotInstance) handleMessage(msg IncomingMessage) {
	// Handle price data — run strategy evaluation
	if len(msg.Prices) > 0 {
		b.handlePrices(msg)
		return
	}

	// Handle portfolio update
	if msg.PortfolioValue > 0 {
		b.mu.Lock()
		b.portfolioValue = msg.PortfolioValue
		b.Position = msg.Position
		b.mu.Unlock()
		return
	}

	// Handle fills
	switch msg.Type {
	case "fill":
		b.handleFill(msg)
	case "ack":
		if msg.ClientID == b.clientID {
			log.Printf("[simbot/%s] ack order=%s status=%s", b.ID, msg.OrderID, msg.Status)
		}
	}
}

func (b *BotInstance) handlePrices(msg IncomingMessage) {
	if len(msg.Prices) == 0 {
		return
	}

	lastClose := msg.Prices[len(msg.Prices)-1].Close

	// Use wall-clock time for bucket alignment; WS timestamps are often 0.
	sampleTime := time.Now()
	if msg.Timestamp > 0 {
		sampleTime = time.UnixMilli(msg.Timestamp)
	}
	actions := b.evaluator.EvaluateSample(lastClose, sampleTime)

	// Execute any actions
	for _, action := range actions {
		orderID := fmt.Sprintf("sbot_%s_%d", b.ID, b.orderCounter.Add(1))

		err := b.client.Send(OutgoingOrder{
			Type:     "market",
			ClientID: b.clientID,
			Symbol:   b.Symbol,
			OrderID:  orderID,
			Side:     action.Side,
			Quantity: action.Quantity,
			Mode:     b.Mode,
			UserID:   b.UserID,
		})

		if err != nil {
			b.addLog(fmt.Sprintf("Order send error: %v", err), "error")
		} else {
			b.applySyntheticFill(action.Side, action.Quantity, lastClose)
			side := "BUY"
			if action.Side == "sell" {
				side = "SELL"
			}
			b.addLog(
				fmt.Sprintf("%s %.4f @ %.2f", side, action.Quantity, lastClose),
				"trade",
			)
		}
	}
	b.revaluePnL(lastClose)
}

func (b *BotInstance) handleFill(msg IncomingMessage) {
	if msg.ClientID != b.clientID {
		return
	}

	b.mu.Lock()
	if msg.Side == "buy" {
		b.Position += msg.Quantity
	} else if msg.Side == "sell" {
		b.Position -= msg.Quantity
	}
	b.mu.Unlock()

	b.addLog(
		fmt.Sprintf("FILL %s %.4f @ %.2f | pos=%.4f",
			msg.Side, msg.Quantity, msg.Price, b.Position),
		"trade",
	)
}

func (b *BotInstance) applySyntheticFill(side string, qty float64, px float64) {
	if qty <= 0 || px <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	// If reducing/reversing, realise PnL against current average entry.
	if b.Position > 0 && side == "sell" {
		closed := qty
		if closed > b.Position {
			closed = b.Position
		}
		b.Realized += (px - b.AvgEntry) * closed
	} else if b.Position < 0 && side == "buy" {
		closed := qty
		if closed > -b.Position {
			closed = -b.Position
		}
		b.Realized += (b.AvgEntry - px) * closed
	}

	delta := qty
	if side == "sell" {
		delta = -qty
	}
	nextPos := b.Position + delta

	switch {
	case b.Position == 0:
		b.AvgEntry = px
	case (b.Position > 0 && nextPos > 0 && side == "buy") || (b.Position < 0 && nextPos < 0 && side == "sell"):
		totalAbs := math.Abs(b.Position) + math.Abs(delta)
		if totalAbs > 0 {
			b.AvgEntry = (b.AvgEntry*math.Abs(b.Position) + px*math.Abs(delta)) / totalAbs
		}
	case b.Position > 0 && nextPos < 0:
		b.AvgEntry = px
	case b.Position < 0 && nextPos > 0:
		b.AvgEntry = px
	case nextPos == 0:
		b.AvgEntry = 0
	}

	b.Position = nextPos
}

func (b *BotInstance) revaluePnL(mark float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.portfolioValue = b.initialCapital + b.Realized + ((mark - b.AvgEntry) * b.Position)
	b.PnL = b.portfolioValue - b.initialCapital
}

func (b *BotInstance) setStatus(status BotStatus) {
	b.mu.Lock()
	b.Status = status
	b.mu.Unlock()
	b.pushUpdate(nil)
}

func (b *BotInstance) addLog(message string, logType string) {
	entry := BotLog{
		ID:      b.logCounter.Add(1),
		Time:    time.Now().UnixMilli(),
		Message: message,
		Type:    logType,
	}

	b.mu.Lock()
	b.Logs = append(b.Logs, entry)
	if len(b.Logs) > MaxLogs {
		b.Logs = b.Logs[len(b.Logs)-MaxLogs:]
	}
	b.mu.Unlock()

	b.pushUpdate(&entry)

	log.Printf("[simbot/%s] [%s] %s", b.ID, logType, message)
}

func (b *BotInstance) pushUpdate(log *BotLog) {
	if b.updateCh == nil {
		return
	}

	b.mu.RLock()
	update := BotStateUpdate{
		BotID:  b.ID,
		Status: b.Status,
		PnL:    b.PnL,
		Log:    log,
	}
	b.mu.RUnlock()

	// Non-blocking send — drop if channel is full
	select {
	case b.updateCh <- update:
	default:
	}
}
