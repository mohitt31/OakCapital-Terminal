package candle

import (
	"context"
	"log"
	"time"

	"synthbull/internal/eventbus"
)

// Builder consumes trade events from Redis streams, aggregates them into
// OHLCV candles, and publishes closed candles to Redis for DB persistence.
//
// Each symbol runs in its own goroutine with independent state — there is no
// cross-symbol locking. Only closed candles are published to Redis; the WS hub
// has its own in-memory CandleBuilder for real-time partial updates.
type Builder struct {
	sub       *eventbus.Subscriber
	pub       *eventbus.Publisher
	intervals []string
}

// symbolState holds per-symbol candle aggregation.
// Owned exclusively by one goroutine — no synchronization needed.
type symbolState struct {
	candles map[string]*eventbus.CandleEvent
}

func NewBuilder(sub *eventbus.Subscriber, pub *eventbus.Publisher) *Builder {
	return &Builder{
		sub:       sub,
		pub:       pub,
		intervals: []string{"1s"},
	}
}

func (b *Builder) Start(ctx context.Context, symbols []string) {
	for _, sym := range symbols {
		s := sym
		state := &symbolState{candles: make(map[string]*eventbus.CandleEvent)}
		b.sub.ConsumeTradesGroup(ctx, s, "candle-builder", "cb-node-0",
			func(ctx context.Context, ev eventbus.TradeEvent) error {
				return b.processTrade(ctx, state, ev)
			})
	}
	log.Printf("[candle] builder started for %d symbols × %d intervals", len(symbols), len(b.intervals))
}

func (b *Builder) processTrade(ctx context.Context, state *symbolState, trade eventbus.TradeEvent) error {
	for _, interval := range b.intervals {
		key := trade.Symbol + ":" + interval
		ts := truncateToInterval(trade.ExecutedAt, interval)

		c, exists := state.candles[key]
		if !exists || !c.Timestamp.Equal(ts) {
			if exists {
				c.IsClosed = true
				if err := b.pub.PublishCandle(ctx, *c); err != nil {
					log.Printf("[candle] publish closed %s/%s failed: %v", trade.Symbol, interval, err)
				}
			}

			state.candles[key] = &eventbus.CandleEvent{
				Symbol:    trade.Symbol,
				Interval:  interval,
				Open:      trade.Price,
				High:      trade.Price,
				Low:       trade.Price,
				Close:     trade.Price,
				Volume:    trade.Quantity,
				Timestamp: ts,
				IsClosed:  false,
			}
		} else {
			if trade.Price > c.High {
				c.High = trade.Price
			}
			if trade.Price < c.Low {
				c.Low = trade.Price
			}
			c.Close = trade.Price
			c.Volume += trade.Quantity
		}
	}
	return nil
}

func truncateToInterval(t time.Time, interval string) time.Time {
	t = t.UTC()
	switch interval {
	case "1s":
		return t.Truncate(time.Second)
	case "5s":
		return t.Truncate(5 * time.Second)
	case "1m":
		return t.Truncate(time.Minute)
	case "5m":
		return t.Truncate(5 * time.Minute)
	default:
		return t.Truncate(time.Minute)
	}
}
