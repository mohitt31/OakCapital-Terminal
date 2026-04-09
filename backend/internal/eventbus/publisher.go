package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Redis key templates
// ---------------------------------------------------------------------------

const (
	// --- Redis Streams (durable) ---
	StreamTrades  = "events:trades:stream:%s"        // %s = symbol
	StreamOrders  = "events:orders:stream"            // global
	StreamCandles = "events:candle:stream:%s:%s"      // %s = symbol, %s = interval
	StreamErrors  = "events:error:stream"             // global durable error log
	StreamAlerts    = "events:alerts:stream"
	StreamBotStatus = "events:bot:stream"

	// --- Redis Pub/Sub (ephemeral) ---
	PubSubOrderUser  = "events:orders:pubsub:%s"    // %s = userID
	PubSubDepth      = "events:depth:pubsub:%s"     // %s = symbol
	PubSubTicker     = "events:ticker:pubsub:%s"    // %s = symbol
	PubSubCandle     = "events:candle:pubsub:%s:%s" // %s = symbol, %s = interval
	PubSubGBM        = "events:gbm:pubsub:%s"       // %s = symbol
	PubSubPortfolio  = "events:portfolio:pubsub:%s" // %s = userID
	PubSubAlert      = "events:alert:pubsub:%s"     // %s = userID
	PubSubBotStatus  = "events:bot:pubsub:%s"       // %s = botID
	PubSubHealth     = "events:health:pubsub:system" // singleton system channel
)

// ---------------------------------------------------------------------------
// Publisher
// ---------------------------------------------------------------------------

// Event publisher
type Publisher struct {
	r *redis.Client
}

// New publisher
func NewPublisher(c *Client) *Publisher {
	return &Publisher{r: c.Redis}
}

// --- Durable (Stream) publishes ---

// Publish trade
func (p *Publisher) PublishTrade(ctx context.Context, ev TradeEvent) error {
	return p.xadd(ctx, fmt.Sprintf(StreamTrades, ev.Symbol), ev)
}

// Publish order
func (p *Publisher) PublishOrderUpdate(ctx context.Context, ev OrderUpdateEvent) error {
	if err := p.xadd(ctx, StreamOrders, ev); err != nil {
		return fmt.Errorf("PublishOrderUpdate stream: %w", err)
	}
	if ev.UserID != "" {
		if err := p.pubsub(ctx, fmt.Sprintf(PubSubOrderUser, ev.UserID), ev); err != nil {
			return fmt.Errorf("PublishOrderUpdate pubsub: %w", err)
		}
	}
	return nil
}

// Publish candle
func (p *Publisher) PublishCandle(ctx context.Context, ev CandleEvent) error {
	if err := p.pubsub(ctx, fmt.Sprintf(PubSubCandle, ev.Symbol, ev.Interval), ev); err != nil {
		return fmt.Errorf("PublishCandle pubsub: %w", err)
	}
	if ev.IsClosed {
		if err := p.xadd(ctx, fmt.Sprintf(StreamCandles, ev.Symbol, ev.Interval), ev); err != nil {
			return fmt.Errorf("PublishCandle stream: %w", err)
		}
	}
	return nil
}

// --- Ephemeral (Pub/Sub) publishes ---

// Publish depth
func (p *Publisher) PublishDepth(ctx context.Context, ev DepthEvent) error {
	return p.pubsub(ctx, fmt.Sprintf(PubSubDepth, ev.Symbol), ev)
}

// Publish ticker
func (p *Publisher) PublishTicker(ctx context.Context, ev TickerEvent) error {
	return p.pubsub(ctx, fmt.Sprintf(PubSubTicker, ev.Symbol), ev)
}

// Publish tick
func (p *Publisher) PublishGBMTick(ctx context.Context, ev GBMTickEvent) error {
	return p.pubsub(ctx, fmt.Sprintf(PubSubGBM, ev.Symbol), ev)
}

// --- User & Portfolio (Pub/Sub) ---

// Publish portfolio
func (p *Publisher) PublishPortfolio(ctx context.Context, ev PortfolioEvent) error {
	return p.pubsub(ctx, fmt.Sprintf(PubSubPortfolio, ev.UserID), ev)
}

// Publish alert
func (p *Publisher) PublishAlert(ctx context.Context, ev AlertEvent) error {
	_ = p.xadd(ctx, StreamAlerts, ev) // durable
	return p.pubsub(ctx, fmt.Sprintf(PubSubAlert, ev.UserID), ev) // real-time
}

// --- Bot Lifecycle (Pub/Sub) ---

// Publish status
func (p *Publisher) PublishBotStatus(ctx context.Context, ev BotStatusEvent) error {
	_ = p.xadd(ctx, StreamBotStatus, ev) // durable
	return p.pubsub(ctx, fmt.Sprintf(PubSubBotStatus, ev.BotID), ev) // real-time
}

// --- System / Operational ---

// Publish health
func (p *Publisher) PublishHealth(ctx context.Context, ev HealthEvent) error {
	return p.pubsub(ctx, PubSubHealth, ev)
}

// Publish error
func (p *Publisher) PublishError(ctx context.Context, ev ErrorEvent) error {
	return p.xadd(ctx, StreamErrors, ev)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (p *Publisher) xadd(ctx context.Context, stream string, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("eventbus marshal for stream %q: %w", stream, err)
	}
	return p.r.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: 100_000,
		Approx: true,
		ID:     "*",
		Values: map[string]any{"p": string(payload)},
	}).Err()
}

func (p *Publisher) pubsub(ctx context.Context, channel string, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("eventbus marshal for channel %q: %w", channel, err)
	}
	return p.r.Publish(ctx, channel, payload).Err()
}
