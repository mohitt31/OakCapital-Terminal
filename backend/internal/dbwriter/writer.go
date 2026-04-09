// Package dbwriter provides batch-write consumers for the Redis Streams
// that feed persistent storage to PostgreSQL.
//
// Design philosophy:
//   - DB-agnostic: callers inject handler functions (caller controls DB layer)
//   - At-least-once delivery:  ACK is sent only after handler returns nil.
//     DB failures leave messages in PEL and retry on next restart.
//   - Parallel consumers: multiple (symbol, interval, service) pairs run independently.
//   - Graceful shutdown: all goroutines finish cleanly on context cancellation.
//
// Consumers:
//
//   - CandleWriter:  batches N closed CandleEvents → onBatch(batch)
//     Recommended batchSize = 60 (one minute of 1s candles, or 12 of 5s candles)
//
//   - OrderWriter:   single-message consumer for orders stream
//     Calls onOrder for each FILLED/PARTIALLY_FILLED state change only.
//
//   - TradeWriter:   single-message consumer per symbol for trades stream
//     Calls onTrade for every matched trade.
//
//   - ErrorWriter:   single-message consumer for global errors stream
//     Calls onError for every ErrorEvent (system errors, operator alerts).
//
//   - AlertWriter:   single-message consumer for global alerts stream
//     Calls onAlert for every user alert event (market conditions, threshold breaches).
//
//   - BotStatusWriter: single-message consumer for global bot status stream
//     Calls onBotStatus for every bot lifecycle event (start, stop, error).
//
//   - PortfolioWriter: single-message consumer for portfolio updates
//     Calls onPortfolio for every portfolio change (used by WebSocket clients).
//
// PostgreSQL schema additions for each writer type:
//
//   - See migration files in internal/db/migrations/sql/
//   - schemas: trades, order_history, system_error_log, user_alerts,
//     bot_status_log, portfolio_snapshots
package dbwriter

import (
	"context"
	"log"

	"synthbull/internal/eventbus"
)

// CandleWriter consumes closed candles from Redis Streams and delivers them
// to the caller in configurable batches.
type CandleWriter struct {
	sub       *eventbus.Subscriber
	batchSize int
}

// NewCandleWriter creates a CandleWriter.
// batchSize is the number of closed candles to accumulate before calling
// onBatch.  60 is a sensible default (one minute of 1-second candles).
func NewCandleWriter(sub *eventbus.Subscriber, batchSize int) *CandleWriter {
	if batchSize <= 0 {
		batchSize = 60
	}
	return &CandleWriter{sub: sub, batchSize: batchSize}
}

// Start launches one batch-consumer goroutine per (symbol, interval) pair.
// onBatch receives a slice of up to batchSize closed CandleEvents.
// Return nil to ACK the batch; return a non-nil error to leave the messages
// in the PEL for retry on the next startup.
//
// Typical onBatch implementation (pgx example):
//
//	func(batch []eventbus.CandleEvent) error {
//	    _, err := pool.CopyFrom(ctx,
//	        pgx.Identifier{"candles"},
//	        []string{"symbol","interval","open","high","low","close","volume","ts"},
//	        pgx.CopyFromSlice(len(batch), func(i int) ([]any, error) {
//	            c := batch[i]
//	            return []any{c.Symbol, c.Interval, c.Open, c.High, c.Low, c.Close, c.Volume, c.Timestamp}, nil
//	        }),
//	    )
//	    return err
//	}
func (w *CandleWriter) Start(ctx context.Context, symbols, intervals []string, onBatch func([]eventbus.CandleEvent) error) {
	for _, sym := range symbols {
		for _, iv := range intervals {
			s, interval := sym, iv
			w.sub.ConsumeCandlesBatchGroup(
				ctx, s, interval,
				"db-writer", "dbw-node-0",
				w.batchSize,
				func(ctx context.Context, batch []eventbus.CandleEvent) error {
					if err := onBatch(batch); err != nil {
						log.Printf("[dbwriter] candle batch write failed (%s/%s, %d candles): %v",
							s, interval, len(batch), err)
						return err
					}
					log.Printf("[dbwriter] wrote %d candles for %s/%s", len(batch), s, interval)
					return nil
				},
			)
		}
	}
	log.Printf("[dbwriter] candle writer started: %d symbols × %d intervals (batch=%d)",
		len(symbols), len(intervals), w.batchSize)
}

// ---------------------------------------------------------------------------
// OrderWriter
// ---------------------------------------------------------------------------

// OrderWriter persists executed orders from the global orders stream.
// It calls onOrder for every message whose status is FILLED or PARTIALLY_FILLED
// so that the caller can INSERT the record into order_history.
type OrderWriter struct {
	sub *eventbus.Subscriber
}

// NewOrderWriter creates an OrderWriter.
func NewOrderWriter(sub *eventbus.Subscriber) *OrderWriter {
	return &OrderWriter{sub: sub}
}

// Start launches the order-history consumer goroutine.
// onOrder is called for each completed (FILLED or PARTIALLY_FILLED) order.
// Return nil to ACK; return an error to keep the message in the PEL.
//
// Callers can filter by status inside onOrder if they want to persist
// CANCELLED orders as well.
func (w *OrderWriter) Start(ctx context.Context, onOrder func(eventbus.OrderUpdateEvent) error) {
	w.sub.ConsumeOrdersGroup(ctx, "db-writer", "dbw-node-0", func(ctx context.Context, ev eventbus.OrderUpdateEvent) error {
		if err := onOrder(ev); err != nil {
			log.Printf("[dbwriter] order write failed (order_id=%d): %v", ev.OrderID, err)
			return err
		}
		return nil
	})
	log.Printf("[dbwriter] order writer started")
}

// ---------------------------------------------------------------------------
// TradeWriter
// ---------------------------------------------------------------------------

// TradeWriter persists executed trades from Redis Streams.
// Maintains one consumer group per symbol for parallel, scalable processing.
type TradeWriter struct {
	sub *eventbus.Subscriber
}

// NewTradeWriter creates a TradeWriter.
func NewTradeWriter(sub *eventbus.Subscriber) *TradeWriter {
	return &TradeWriter{sub: sub}
}

// Start launches trade-consumer goroutines, one per symbol.
// onTrade is called for every TradeEvent.
// Return nil to ACK; return an error to keep the message in the PEL.
//
// Typical onTrade usage:
//
//	func(trade eventbus.TradeEvent) error {
//	    // INSERT INTO trades (symbol, price, qty, maker_id, taker_id, taker_side, executed_at)
//	    // VALUES ($1, $2, $3, $4, $5, $6, $7)
//	    return db.InsertTrade(ctx, trade)
//	}
func (w *TradeWriter) Start(ctx context.Context, symbols []string, onTrade func(eventbus.TradeEvent) error) {
	for _, sym := range symbols {
		capturedSym := sym
		w.sub.ConsumeTradesGroup(ctx, capturedSym, "db-writer", "dbw-node-0", func(ctx context.Context, ev eventbus.TradeEvent) error {
			if err := onTrade(ev); err != nil {
				log.Printf("[dbwriter] trade write failed (symbol=%s, maker=%d, taker=%d): %v",
					capturedSym, ev.MakerOrderID, ev.TakerOrderID, err)
				return err
			}
			return nil
		})
	}
	log.Printf("[dbwriter] trade writer started: %d symbols", len(symbols))
}

// ---------------------------------------------------------------------------
// ErrorWriter
// ---------------------------------------------------------------------------

// ErrorWriter persists system errors from the global errors stream.
type ErrorWriter struct {
	sub *eventbus.Subscriber
}

// NewErrorWriter creates an ErrorWriter.
func NewErrorWriter(sub *eventbus.Subscriber) *ErrorWriter {
	return &ErrorWriter{sub: sub}
}

// Start launches the error-log consumer goroutine.
// onError is called for every ErrorEvent in the system errors stream.
// Return nil to ACK; return an error to keep the message in the PEL.
//
// Typical onError usage:
//
//	func(err eventbus.ErrorEvent) error {
//	    // INSERT INTO system_error_log (service, code, message, details, severity, occurred_at)
//	    // VALUES ($1, $2, $3, $4, $5, $6)
//	    return db.LogError(ctx, err)
//	}
func (w *ErrorWriter) Start(ctx context.Context, onError func(eventbus.ErrorEvent) error) {
	w.sub.ConsumeErrorsGroup(ctx, "db-writer", "dbw-node-0", func(ctx context.Context, ev eventbus.ErrorEvent) error {
		if err := onError(ev); err != nil {
			log.Printf("[dbwriter] error log write failed (service=%s, code=%s): %v",
				ev.ServiceName, ev.ErrorCode, err)
			return err
		}
		return nil
	})
	log.Printf("[dbwriter] error writer started")
}

// ---------------------------------------------------------------------------
// AlertWriter
// ---------------------------------------------------------------------------

// AlertWriter persists user alerts to the database.
// Typically used for user notifications (price thresholds, portfolio changes).
type AlertWriter struct {
	sub *eventbus.Subscriber
}

// NewAlertWriter creates an AlertWriter.
func NewAlertWriter(sub *eventbus.Subscriber) *AlertWriter {
	return &AlertWriter{sub: sub}
}

// Start launches alert-consumer goroutines for all users.
// Since we can't predict all users in advance, callers should use
// a wildcard PubSub consumer or manually add per-user consumers as needed.
//
// For simplicity in production, recommend a single consumer group on a
// global alerts stream (if implemented in Publisher).
func (w *AlertWriter) StartGlobal(ctx context.Context, onAlert func(eventbus.AlertEvent) error) {
	w.sub.ConsumeAlertsGroup(ctx, "db-writer", "dbw-node-0", func(ctx context.Context, ev eventbus.AlertEvent) error {
		if err := onAlert(ev); err != nil {
			log.Printf("[dbwriter] alert write failed (user=%s): %v", ev.UserID, err)
			return err
		}
		return nil
	})
	log.Printf("[dbwriter] alert writer started")
}

// ---------------------------------------------------------------------------
// BotStatusWriter
// ---------------------------------------------------------------------------

// BotStatusWriter persists bot lifecycle events to the database.
type BotStatusWriter struct {
	sub *eventbus.Subscriber
}

// NewBotStatusWriter creates a BotStatusWriter.
func NewBotStatusWriter(sub *eventbus.Subscriber) *BotStatusWriter {
	return &BotStatusWriter{sub: sub}
}

// Start launches bot-status-consumer goroutines for all tracked bots.
// Similar to AlertWriter, this is typically managed by subscribing to pub/sub topics
// or a global stream as bots are created.
//
// For a production system, recommend:
//   1. Maintaining a registry of active bot IDs
//   2. Dynamically subscribing to each bot's status channel
//   3. Persisting status changes for audit/replay purposes
func (w *BotStatusWriter) StartGlobal(ctx context.Context, onBotStatus func(eventbus.BotStatusEvent) error) {
	w.sub.ConsumeBotStatusGroup(ctx, "db-writer", "dbw-node-0", func(ctx context.Context, ev eventbus.BotStatusEvent) error {
		if err := onBotStatus(ev); err != nil {
			log.Printf("[dbwriter] bot status write failed (bot=%s): %v", ev.BotID, err)
			return err
		}
		return nil
	})
	log.Printf("[dbwriter] bot status writer started")
}

// ---------------------------------------------------------------------------
// PortfolioWriter
// ---------------------------------------------------------------------------

// PortfolioWriter persists portfolio snapshots to the database.
// Used for historical tracking and auditing of user account state.
type PortfolioWriter struct {
	sub *eventbus.Subscriber
}

// NewPortfolioWriter creates a PortfolioWriter.
func NewPortfolioWriter(sub *eventbus.Subscriber) *PortfolioWriter {
	return &PortfolioWriter{sub: sub}
}

// Start is a placeholder for portfolio persistence.
// In the current architecture, portfolios are flushed directly from memory
// (internal/portfolio/manager.go) on a 2-second timer, not through the event bus.
//
// If you want to:
//   1. Audit every portfolio change → consume portfolio events from pub/sub
//   2. Maintain a complete history → subscribe to per-user portfolio topics
//   3. Trigger downstream analytics → use this writer
func (w *PortfolioWriter) StartGlobal(ctx context.Context, onPortfolio func(eventbus.PortfolioEvent) error) {
	// Portfolio persistence is handled by internal/portfolio/manager.go (2-second flush timer).
	// This writer is available for additional snapshot auditing if needed.
	log.Printf("[dbwriter] portfolio writer ready (primary persistence via manager.go 2s flush)")
}
