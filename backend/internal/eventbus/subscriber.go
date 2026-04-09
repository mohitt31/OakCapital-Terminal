package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Busy group
func isBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}

// ---------------------------------------------------------------------------
// Subscriber
// ---------------------------------------------------------------------------

// Event subscriber
type Subscriber struct {
	r  *redis.Client
	wg sync.WaitGroup // tracks all running consumer goroutines
}

// New subscriber
func NewSubscriber(c *Client) *Subscriber {
	return &Subscriber{r: c.Redis}
}

// Wait workers
func (s *Subscriber) Wait() { s.wg.Wait() }

// ============================================================
// Pub/Sub consumers  (ephemeral, real-time — no persistence)
// ============================================================

// Depth updates
// handler is called in a dedicated goroutine; blocks until ctx is cancelled.
func (s *Subscriber) OnDepth(ctx context.Context, symbol string, handler func(ctx context.Context, ev DepthEvent)) {
	s.subscribeChannel(ctx, fmt.Sprintf(PubSubDepth, symbol), func(payload string) {
		var ev DepthEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			log.Printf("[eventbus] unmarshal DepthEvent: %v", err)
			return
		}
		handler(ctx, ev)
	})
}

// Ticker updates
func (s *Subscriber) OnTicker(ctx context.Context, symbol string, handler func(ctx context.Context, ev TickerEvent)) {
	s.subscribeChannel(ctx, fmt.Sprintf(PubSubTicker, symbol), func(payload string) {
		var ev TickerEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			log.Printf("[eventbus] unmarshal TickerEvent: %v", err)
			return
		}
		handler(ctx, ev)
	})
}

// Candle ticks
func (s *Subscriber) OnCandleTick(ctx context.Context, symbol, interval string, handler func(ctx context.Context, ev CandleEvent)) {
	s.subscribeChannel(ctx, fmt.Sprintf(PubSubCandle, symbol, interval), func(payload string) {
		var ev CandleEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			log.Printf("[eventbus] unmarshal CandleEvent (pubsub): %v", err)
			return
		}
		handler(ctx, ev)
	})
}

// Order updates
func (s *Subscriber) OnOrderUpdate(ctx context.Context, userID string, handler func(ctx context.Context, ev OrderUpdateEvent)) {
	s.subscribeChannel(ctx, fmt.Sprintf(PubSubOrderUser, userID), func(payload string) {
		var ev OrderUpdateEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			log.Printf("[eventbus] unmarshal OrderUpdateEvent (pubsub): %v", err)
			return
		}
		handler(ctx, ev)
	})
}

// GBM ticks
func (s *Subscriber) OnGBMTick(ctx context.Context, symbol string, handler func(ctx context.Context, ev GBMTickEvent)) {
	s.subscribeChannel(ctx, fmt.Sprintf(PubSubGBM, symbol), func(payload string) {
		var ev GBMTickEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			log.Printf("[eventbus] unmarshal GBMTickEvent (pubsub): %v", err)
			return
		}
		handler(ctx, ev)
	})
}

// Portfolio updates
func (s *Subscriber) OnPortfolio(ctx context.Context, userID string, handler func(ctx context.Context, ev PortfolioEvent)) {
	s.subscribeChannel(ctx, fmt.Sprintf(PubSubPortfolio, userID), func(payload string) {
		var ev PortfolioEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			log.Printf("[eventbus] unmarshal PortfolioEvent (pubsub): %v", err)
			return
		}
		handler(ctx, ev)
	})
}

// Bot status
func (s *Subscriber) OnBotStatus(ctx context.Context, botID string, handler func(ctx context.Context, ev BotStatusEvent)) {
	s.subscribeChannel(ctx, fmt.Sprintf(PubSubBotStatus, botID), func(payload string) {
		var ev BotStatusEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			log.Printf("[eventbus] unmarshal BotStatusEvent (pubsub): %v", err)
			return
		}
		handler(ctx, ev)
	})
}

// Health events
func (s *Subscriber) OnHealth(ctx context.Context, handler func(ctx context.Context, ev HealthEvent)) {
	s.subscribeChannel(ctx, PubSubHealth, func(payload string) {
		var ev HealthEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			log.Printf("[eventbus] unmarshal HealthEvent (pubsub): %v", err)
			return
		}
		handler(ctx, ev)
	})
}

// Subscribe channel
func (s *Subscriber) subscribeChannel(ctx context.Context, channel string, handle func(string)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		pubsub := s.r.Subscribe(ctx, channel)
		defer pubsub.Close()

		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return // channel closed
				}
				handle(msg.Payload)
			}
		}
	}()
}

// ============================================================
// Stream consumers  (durable, at-least-once delivery)
// ============================================================

// Consume trades
func (s *Subscriber) ConsumeTradesGroup(ctx context.Context, symbol, group, consumer string, handler func(ctx context.Context, ev TradeEvent) error) {
	streamKey := fmt.Sprintf(StreamTrades, symbol)
	s.consumeStream(ctx, streamKey, group, consumer, func(payload string) error {
		var ev TradeEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			return fmt.Errorf("unmarshal TradeEvent: %w", err)
		}
		return handler(ctx, ev)
	})
}

// Consume orders
func (s *Subscriber) ConsumeOrdersGroup(ctx context.Context, group, consumer string, handler func(ctx context.Context, ev OrderUpdateEvent) error) {
	s.consumeStream(ctx, StreamOrders, group, consumer, func(payload string) error {
		var ev OrderUpdateEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			return fmt.Errorf("unmarshal OrderUpdateEvent: %w", err)
		}
		return handler(ctx, ev)
	})
}

// Consume candles
func (s *Subscriber) ConsumeCandlesGroup(ctx context.Context, symbol, interval, group, consumer string, handler func(ctx context.Context, ev CandleEvent) error) {
	streamKey := fmt.Sprintf(StreamCandles, symbol, interval)
	s.consumeStream(ctx, streamKey, group, consumer, func(payload string) error {
		var ev CandleEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			return fmt.Errorf("unmarshal CandleEvent: %w", err)
		}
		return handler(ctx, ev)
	})
}

// Batch candles
func (s *Subscriber) ConsumeCandlesBatchGroup(ctx context.Context, symbol, interval, group, consumer string, batchSize int, handler func(ctx context.Context, batch []CandleEvent) error) {
	streamKey := fmt.Sprintf(StreamCandles, symbol, interval)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		if err := s.ensureGroup(ctx, streamKey, group); err != nil {
			log.Printf("[eventbus] cannot create consumer group on %q: %v", streamKey, err)
			return
		}

		// Phase 1: drain existing PEL into a batch.
		s.drainPELBatch(ctx, streamKey, group, consumer, batchSize, handler)

		// Phase 2: continuous batched polling for new messages.
		var (
			batch   []CandleEvent
			batchIDs []string // stream message IDs corresponding to batch entries
		)

		for {
			select {
			case <-ctx.Done():
				// Flush any partial batch before exiting.
				if len(batch) > 0 {
					if err := handler(ctx, batch); err == nil {
						s.r.XAck(ctx, streamKey, group, batchIDs...)
					}
				}
				return
			default:
			}

			entries, err := s.r.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    group,
				Consumer: consumer,
				Streams:  []string{streamKey, ">"},
				Count:    int64(batchSize),
				Block:    100 * time.Millisecond,
			}).Result()

			if err != nil {
				if errors.Is(err, redis.Nil) {
					// Flush partial batches when the stream is briefly idle so
					// messages are not left pending until process shutdown.
					if len(batch) > 0 {
						if err := handler(ctx, batch); err == nil {
							s.r.XAck(ctx, streamKey, group, batchIDs...)
						}
						batch = batch[:0]
						batchIDs = batchIDs[:0]
					}
					continue
				}
				if ctx.Err() != nil {
					return
				}
				log.Printf("[eventbus] XReadGroup (batch) on %q: %v — retrying in 1s", streamKey, err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
				continue
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					payload, ok := msg.Values["p"].(string)
					if !ok {
						// Malformed — skip and ACK immediately to unblock the PEL.
						s.r.XAck(ctx, streamKey, group, msg.ID)
						continue
					}
					var ev CandleEvent
					if err := json.Unmarshal([]byte(payload), &ev); err != nil {
						log.Printf("[eventbus] unmarshal CandleEvent (batch) msg %s: %v — skipping", msg.ID, err)
						s.r.XAck(ctx, streamKey, group, msg.ID)
						continue
					}
					batch = append(batch, ev)
					batchIDs = append(batchIDs, msg.ID)
				}
			}

			if len(batch) >= batchSize {
				if err := handler(ctx, batch); err != nil {
					log.Printf("[eventbus] batch handler error on %q (%d candles): %v — not acked", streamKey, len(batch), err)
					// Do not ACK — messages stay in PEL for retry on restart.
					batch = batch[:0]
					batchIDs = batchIDs[:0]
					continue
				}
				s.r.XAck(ctx, streamKey, group, batchIDs...)
				batch = batch[:0]
				batchIDs = batchIDs[:0]
			}
		}
	}()
}

// Drain batch
func (s *Subscriber) drainPELBatch(ctx context.Context, streamKey, group, consumer string, batchSize int, handler func(ctx context.Context, batch []CandleEvent) error) {
	startID := "0"
	var batch []CandleEvent
	var batchIDs []string

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		entries, err := s.r.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{streamKey, startID},
			Count:    int64(batchSize),
		}).Result()

		if err != nil || len(entries) == 0 || len(entries[0].Messages) == 0 {
			// PEL drained — flush any partial batch collected so far.
			if len(batch) > 0 {
				if err := handler(ctx, batch); err == nil {
					s.r.XAck(ctx, streamKey, group, batchIDs...)
				}
			}
			return
		}

		msgs := entries[0].Messages
		for _, msg := range msgs {
			payload, ok := msg.Values["p"].(string)
			if !ok {
				s.r.XAck(ctx, streamKey, group, msg.ID)
				continue
			}
			var ev CandleEvent
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				s.r.XAck(ctx, streamKey, group, msg.ID)
				continue
			}
			batch = append(batch, ev)
			batchIDs = append(batchIDs, msg.ID)
		}

		if len(batch) >= batchSize {
			if err := handler(ctx, batch); err == nil {
				s.r.XAck(ctx, streamKey, group, batchIDs...)
			}
			batch = batch[:0]
			batchIDs = batchIDs[:0]
		}

		startID = msgs[len(msgs)-1].ID
	}
}

// Consume errors
func (s *Subscriber) ConsumeErrorsGroup(ctx context.Context, group, consumer string, handler func(ctx context.Context, ev ErrorEvent) error) {
	s.consumeStream(ctx, StreamErrors, group, consumer, func(payload string) error {
		var ev ErrorEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			return fmt.Errorf("unmarshal ErrorEvent: %w", err)
		}
		return handler(ctx, ev)
	})
}

// ConsumeAlertsGroup consumes from the global alerts stream.
func (s *Subscriber) ConsumeAlertsGroup(ctx context.Context, group, consumer string, handler func(ctx context.Context, ev AlertEvent) error) {
	s.consumeStream(ctx, StreamAlerts, group, consumer, func(payload string) error {
		var ev AlertEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			return fmt.Errorf("unmarshal AlertEvent: %w", err)
		}
		return handler(ctx, ev)
	})
}

// ConsumeBotStatusGroup consumes from the global bot status stream.
func (s *Subscriber) ConsumeBotStatusGroup(ctx context.Context, group, consumer string, handler func(ctx context.Context, ev BotStatusEvent) error) {
	s.consumeStream(ctx, StreamBotStatus, group, consumer, func(payload string) error {
		var ev BotStatusEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			return fmt.Errorf("unmarshal BotStatusEvent: %w", err)
		}
		return handler(ctx, ev)
	})
}

// ---------------------------------------------------------------------------
// Consume stream
// ---------------------------------------------------------------------------
func (s *Subscriber) consumeStream(ctx context.Context, streamKey, group, consumer string, handle func(string) error) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Create group
		if err := s.ensureGroup(ctx, streamKey, group); err != nil {
			log.Printf("[eventbus] cannot create consumer group on %q: %v", streamKey, err)
			return
		}

		// Drain PEL
		s.drainPEL(ctx, streamKey, group, consumer, handle)

		// Phase 2: continuous polling for new messages.
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			entries, err := s.r.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    group,
				Consumer: consumer,
				Streams:  []string{streamKey, ">"},
				Count:    50,           // Batch size
				Block:    2 * time.Second,
			}).Result()

			if err != nil {
				if errors.Is(err, redis.Nil) {
					// Block timeout — no new messages, loop around.
					continue
				}
				if ctx.Err() != nil {
					return // context cancelled, clean exit
				}
				log.Printf("[eventbus] XReadGroup on %q: %v — retrying in 1s", streamKey, err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
				continue
			}

			s.processEntries(ctx, streamKey, group, entries, handle)
		}
	}()
}

// Ensure group
func (s *Subscriber) ensureGroup(ctx context.Context, streamKey, group string) error {
	// Deliver early
	err := s.r.XGroupCreateMkStream(ctx, streamKey, group, "0").Err()
	if err != nil && !isBusyGroup(err) {
		return err
	}
	return nil
}

// Drain PEL
func (s *Subscriber) drainPEL(ctx context.Context, streamKey, group, consumer string, handle func(string) error) {
	startID := "0" // Oldest unacked
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		entries, err := s.r.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{streamKey, startID},
			Count:    50,
		}).Result()

		if err != nil || len(entries) == 0 || len(entries[0].Messages) == 0 {
			// PEL is empty — proceed to normal consumption.
			return
		}

		msgs := entries[0].Messages
		s.processEntries(ctx, streamKey, group, entries, handle)

		// Advance past the last message we processed.
		startID = msgs[len(msgs)-1].ID
	}
}

// Process entries
func (s *Subscriber) processEntries(ctx context.Context, streamKey, group string, entries []redis.XStream, handle func(string) error) {
	for _, stream := range entries {
		for _, msg := range stream.Messages {
			payload, ok := msg.Values["p"].(string)
			if !ok {
				// ACK malformed
				log.Printf("[eventbus] unexpected payload type in %q (msg %s) — acking to skip", streamKey, msg.ID)
				s.r.XAck(ctx, streamKey, group, msg.ID)
				continue
			}

			if err := handle(payload); err != nil {
				// Retry later
				log.Printf("[eventbus] handler error on msg %s in %q (not acked): %v", msg.ID, streamKey, err)
				continue
			}

			// ACK success
			if err := s.r.XAck(ctx, streamKey, group, msg.ID).Err(); err != nil {
				log.Printf("[eventbus] XAck failed for msg %s in %q: %v", msg.ID, streamKey, err)
			}
		}
	}
}
