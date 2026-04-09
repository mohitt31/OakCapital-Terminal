package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"synthbull/pkg/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestClient spins up an in-process Redis (miniredis) and returns a Client wired to it.
func newTestClient(t *testing.T) (*miniredis.Miniredis, *Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, &Client{Redis: rdb}
}

// readStreamOnce polls a Stream Consumer Group non-blocking (miniredis doesn't support
// blocking XREADGROUP). It uses the "p" field set by our xadd helper.
func readStreamOnce(t *testing.T, ctx context.Context, c *Client, stream, group, consumer string) string {
	t.Helper()

	if err := c.Redis.XGroupCreateMkStream(ctx, stream, group, "0").Err(); err != nil {
		if !isGroupExists(err) {
			t.Fatalf("create group: %v", err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := c.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    1,
			Block:    0,
		}).Result()

		if err == redis.Nil || len(entries) == 0 || len(entries[0].Messages) == 0 {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if err != nil {
			t.Fatalf("XReadGroup: %v", err)
		}

		msg := entries[0].Messages[0]
		c.Redis.XAck(ctx, stream, group, msg.ID)

		payload, ok := msg.Values["p"].(string)
		if !ok {
			t.Fatalf(`field "p" is not a string: %T`, msg.Values["p"])
		}
		return payload
	}
	t.Fatal("timed out waiting for stream message")
	return ""
}

func isGroupExists(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}

func TestPubSubDepth(t *testing.T) {
	mr, client := newTestClient(t)
	defer mr.Close()
	defer client.Close()

	pub := NewPublisher(client)
	sub := NewSubscriber(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		wg       sync.WaitGroup
		received DepthEvent
	)
	wg.Add(1)

	sub.OnDepth(ctx, "BTC", func(_ context.Context, ev DepthEvent) {
		received = ev
		wg.Done()
	})

	time.Sleep(100 * time.Millisecond)

	want := DepthEvent{
		Symbol:     "BTC",
		Bids:       []models.PriceLevel{{Price: 6000000, Quantity: 150}},
		IsSnapshot: true,
		Timestamp:  time.Now(),
	}
	if err := pub.PublishDepth(ctx, want); err != nil {
		t.Fatalf("PublishDepth: %v", err)
	}

	wg.Wait()
	if len(received.Bids) == 0 || received.Bids[0].Price != 6000000 {
		t.Errorf("unexpected depth received: %+v", received)
	}
}

func TestPubSubTicker(t *testing.T) {
	mr, client := newTestClient(t)
	defer mr.Close()
	defer client.Close()

	pub := NewPublisher(client)
	sub := NewSubscriber(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		wg       sync.WaitGroup
		received TickerEvent
	)
	wg.Add(1)

	sub.OnTicker(ctx, "ETH", func(_ context.Context, ev TickerEvent) {
		received = ev
		wg.Done()
	})

	time.Sleep(100 * time.Millisecond)

	want := TickerEvent{Symbol: "ETH", LastPrice: 340050}
	if err := pub.PublishTicker(ctx, want); err != nil {
		t.Fatalf("PublishTicker: %v", err)
	}

	wg.Wait()
	if received.LastPrice != 340050 {
		t.Errorf("unexpected ticker: %+v", received)
	}
}

func TestStreamTrade(t *testing.T) {
	mr, client := newTestClient(t)
	defer mr.Close()
	defer client.Close()

	pub := NewPublisher(client)
	ctx := context.Background()

	want := TradeEvent{
		Symbol:       "ETH",
		Price:        305000,
		Quantity:     2,
		MakerOrderID: 10,
		TakerOrderID: 11,
		TakerSide:    models.SideBuy,
		ExecutedAt:   time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := pub.PublishTrade(ctx, want); err != nil {
		t.Fatalf("PublishTrade: %v", err)
	}

	streamKey := fmt.Sprintf(StreamTrades, want.Symbol)
	payload := readStreamOnce(t, ctx, client, streamKey, "db-writer", "worker-0")

	var got TradeEvent
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Price != 305000 || got.TakerSide != models.SideBuy {
		t.Errorf("unexpected trade: %+v", got)
	}
}

func TestStreamOrders(t *testing.T) {
	mr, client := newTestClient(t)
	defer mr.Close()
	defer client.Close()

	pub := NewPublisher(client)
	ctx := context.Background()

	want := OrderUpdateEvent{
		OrderID:   99,
		UserID:    "user-123",
		Symbol:    "BTC",
		Side:      models.SideSell,
		Status:    models.StatusFilled,
		Quantity:  1,
		FilledQty: 1,
		AvgPrice:  6050000,
	}
	if err := pub.PublishOrderUpdate(ctx, want); err != nil {
		t.Fatalf("PublishOrderUpdate: %v", err)
	}

	payload := readStreamOnce(t, ctx, client, StreamOrders, "db-writer", "worker-0")

	var got OrderUpdateEvent
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.OrderID != 99 || got.Status != models.StatusFilled {
		t.Errorf("unexpected order update: %+v", got)
	}
}

func TestStreamCandleClosed(t *testing.T) {
	mr, client := newTestClient(t)
	defer mr.Close()
	defer client.Close()

	pub := NewPublisher(client)
	ctx := context.Background()

	want := CandleEvent{
		Symbol:   "SOL",
		Interval: "1s",
		Open:     14500, High: 14600, Low: 14490, Close: 14550,
		Volume:   120,
		IsClosed: true,
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}
	if err := pub.PublishCandle(ctx, want); err != nil {
		t.Fatalf("PublishCandle: %v", err)
	}

	streamKey := fmt.Sprintf(StreamCandles, want.Symbol, want.Interval)
	payload := readStreamOnce(t, ctx, client, streamKey, "db-writer", "worker-0")

	var got CandleEvent
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Close != 14550 || !got.IsClosed {
		t.Errorf("unexpected candle: %+v", got)
	}
}
