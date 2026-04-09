package eventbus

import (
	"fmt"
	"sync"
	"time"
)

// CandleBuilder turns trade (and optional GBM) ticks into CandleEvent streams per symbol and interval.
// It is safe for concurrent use. Wire-up to the event bus and persistence is left to callers.
type CandleBuilder struct {
	intervals []intervalSpec
	shards    []symbolShard
	staleTTL  time.Duration
}

type candleKey struct {
	symbol   string
	interval string
}

type candleAgg struct {
	bucketStart time.Time
	open        int64
	high        int64
	low         int64
	close       int64
	volume      int64
	lastUpdated time.Time
	initialized bool
}

type intervalSpec struct {
	label string
	d     time.Duration
	kind  intervalKind
}

type intervalKind uint8

const (
	intervalKindFixed intervalKind = iota
	intervalKindWeekly
)

type symbolShard struct {
	mu    sync.Mutex
	state map[candleKey]candleAgg
}

// NewCandleBuilder builds candles for the given interval labels (e.g. "1m", "5m", "1h", "1d").
// Unknown labels make construction fail.
func NewCandleBuilder(intervalLabels []string) (*CandleBuilder, error) {
	specs := make([]intervalSpec, 0, len(intervalLabels))
	for _, label := range intervalLabels {
		spec, err := parseInterval(label)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	const shardCount = 64
	shards := make([]symbolShard, shardCount)
	for i := range shards {
		shards[i].state = make(map[candleKey]candleAgg)
	}
	return &CandleBuilder{
		intervals: specs,
		shards:    shards,
	}, nil
}

// SetStaleTTL configures when inactive symbol/interval state should be cleaned.
// A zero or negative value disables TTL-based cleanup.
func (b *CandleBuilder) SetStaleTTL(ttl time.Duration) {
	b.staleTTL = ttl
}

// OnTrade applies a trade to all configured intervals. It returns closed candles first (when a bucket
// rolls), then one partial update per interval for the active bucket (IsClosed=false).
func (b *CandleBuilder) OnTrade(t TradeEvent) []CandleEvent {
	if t.Symbol == "" || t.Quantity <= 0 {
		return nil
	}
	ts := t.ExecutedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	} else {
		ts = ts.UTC()
	}

	shard := b.shardForSymbol(t.Symbol)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	out := make([]CandleEvent, 0, len(b.intervals)*2)
	for _, spec := range b.intervals {
		out = b.applyPriceVolume(out, shard.state, t.Symbol, spec, ts, t.Price, t.Quantity)
	}
	return out
}

// OnGBMTick updates last price from simulation without adding volume. Same emission pattern as OnTrade.
func (b *CandleBuilder) OnGBMTick(e GBMTickEvent) []CandleEvent {
	if e.Symbol == "" {
		return nil
	}
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	} else {
		ts = ts.UTC()
	}

	shard := b.shardForSymbol(e.Symbol)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	out := make([]CandleEvent, 0, len(b.intervals)*2)
	for _, spec := range b.intervals {
		out = b.applyPriceOnly(out, shard.state, e.Symbol, spec, ts, e.BasePrice)
	}
	return out
}

// Flush emits one closed CandleEvent per open bucket and clears builder state. Use during shutdown so the last
// partial candle can be persisted or broadcast as final.
func (b *CandleBuilder) Flush() []CandleEvent {
	out := make([]CandleEvent, 0, len(b.intervals)*len(b.shards))
	for i := range b.shards {
		shard := &b.shards[i]
		shard.mu.Lock()
		for key, agg := range shard.state {
			if agg.initialized {
				out = append(out, closedCandleEvent(key.symbol, key.interval, agg))
			}
			delete(shard.state, key)
		}
		shard.mu.Unlock()
	}
	return out
}

// CleanupStale removes state entries older than now-staleTTL and returns number deleted.
// If stale TTL is not configured, this is a no-op.
func (b *CandleBuilder) CleanupStale(now time.Time) int {
	if b.staleTTL <= 0 {
		return 0
	}
	return b.CleanupBefore(now.Add(-b.staleTTL))
}

// CleanupBefore removes entries whose last update is before the given cutoff.
func (b *CandleBuilder) CleanupBefore(cutoff time.Time) int {
	removed := 0
	for i := range b.shards {
		shard := &b.shards[i]
		shard.mu.Lock()
		for key, agg := range shard.state {
			if agg.lastUpdated.Before(cutoff) {
				delete(shard.state, key)
				removed++
			}
		}
		shard.mu.Unlock()
	}
	return removed
}

func (b *CandleBuilder) applyPriceVolume(out []CandleEvent, state map[candleKey]candleAgg, symbol string, spec intervalSpec, ts time.Time, price, qty int64) []CandleEvent {
	start := alignBucketStart(ts, spec)
	key := candleKey{symbol: symbol, interval: spec.label}
	agg, exists := state[key]

	if exists && !agg.bucketStart.Equal(start) {
		if agg.initialized {
			out = append(out, closedCandleEvent(symbol, spec.label, agg))
		}
		delete(state, key)
		exists = false
		agg = candleAgg{}
	}
	if !exists {
		agg = candleAgg{
			bucketStart: start,
			open:        price,
			high:        price,
			low:         price,
			close:       price,
			volume:      qty,
			lastUpdated: ts,
			initialized: true,
		}
	} else {
		updateOHLCV(&agg, price, qty)
		agg.lastUpdated = ts
	}
	state[key] = agg
	out = append(out, partialCandleEvent(symbol, spec.label, agg))
	return out
}

func (b *CandleBuilder) applyPriceOnly(out []CandleEvent, state map[candleKey]candleAgg, symbol string, spec intervalSpec, ts time.Time, price int64) []CandleEvent {
	start := alignBucketStart(ts, spec)
	key := candleKey{symbol: symbol, interval: spec.label}
	agg, exists := state[key]

	if exists && !agg.bucketStart.Equal(start) {
		if agg.initialized {
			out = append(out, closedCandleEvent(symbol, spec.label, agg))
		}
		delete(state, key)
		exists = false
		agg = candleAgg{}
	}
	if !exists {
		agg = candleAgg{
			bucketStart: start,
			open:        price,
			high:        price,
			low:         price,
			close:       price,
			volume:      0,
			lastUpdated: ts,
			initialized: true,
		}
	} else {
		updateOHLC(&agg, price)
		agg.lastUpdated = ts
	}
	state[key] = agg
	out = append(out, partialCandleEvent(symbol, spec.label, agg))
	return out
}

func updateOHLCV(agg *candleAgg, price, qty int64) {
	if !agg.initialized {
		agg.open, agg.high, agg.low, agg.close = price, price, price, price
		agg.volume = qty
		agg.initialized = true
		return
	}
	if price > agg.high {
		agg.high = price
	}
	if price < agg.low {
		agg.low = price
	}
	agg.close = price
	agg.volume += qty
}

// updateOHLC is used for GBM price ticks (not real trades).
// It only updates Close (current price). High and Low are NOT touched
// because GBM ticks are quotes, not trades — expanding high/low from
// quotes makes candles unrealistically tall.
func updateOHLC(agg *candleAgg, price int64) {
	if !agg.initialized {
		agg.open, agg.high, agg.low, agg.close = price, price, price, price
		agg.initialized = true
		return
	}
	agg.close = price
}

func closedCandleEvent(symbol, label string, agg candleAgg) CandleEvent {
	return CandleEvent{
		Symbol:    symbol,
		Interval:  label,
		Open:      agg.open,
		High:      agg.high,
		Low:       agg.low,
		Close:     agg.close,
		Volume:    agg.volume,
		IsClosed:  true,
		Timestamp: agg.bucketStart,
	}
}

func partialCandleEvent(symbol, label string, agg candleAgg) CandleEvent {
	return CandleEvent{
		Symbol:    symbol,
		Interval:  label,
		Open:      agg.open,
		High:      agg.high,
		Low:       agg.low,
		Close:     agg.close,
		Volume:    agg.volume,
		IsClosed:  false,
		Timestamp: agg.bucketStart,
	}
}

func alignBucketStart(t time.Time, spec intervalSpec) time.Time {
	if spec.kind == intervalKindWeekly {
		return alignWeekMondayUTC(t)
	}
	unix := t.UnixNano()
	step := int64(spec.d)
	if step <= 0 {
		return t.UTC().Truncate(time.Minute)
	}
	bucket := (unix / step) * step
	return time.Unix(0, bucket).UTC()
}

func alignWeekMondayUTC(t time.Time) time.Time {
	u := t.UTC()
	y, m, d := u.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	// Monday-start week: Mon=0 ... Sun=6
	daysSinceMonday := (int(midnight.Weekday()) + 6) % 7
	return midnight.AddDate(0, 0, -daysSinceMonday)
}

func parseInterval(label string) (intervalSpec, error) {
	switch label {
	case "1s":
		return intervalSpec{label: label, d: time.Second, kind: intervalKindFixed}, nil
	case "5s":
		return intervalSpec{label: label, d: 5 * time.Second, kind: intervalKindFixed}, nil
	case "15s":
		return intervalSpec{label: label, d: 15 * time.Second, kind: intervalKindFixed}, nil
	case "30s":
		return intervalSpec{label: label, d: 30 * time.Second, kind: intervalKindFixed}, nil
	case "1m":
		return intervalSpec{label: label, d: time.Minute, kind: intervalKindFixed}, nil
	case "3m":
		return intervalSpec{label: label, d: 3 * time.Minute, kind: intervalKindFixed}, nil
	case "5m":
		return intervalSpec{label: label, d: 5 * time.Minute, kind: intervalKindFixed}, nil
	case "15m":
		return intervalSpec{label: label, d: 15 * time.Minute, kind: intervalKindFixed}, nil
	case "30m":
		return intervalSpec{label: label, d: 30 * time.Minute, kind: intervalKindFixed}, nil
	case "1h":
		return intervalSpec{label: label, d: time.Hour, kind: intervalKindFixed}, nil
	case "2h":
		return intervalSpec{label: label, d: 2 * time.Hour, kind: intervalKindFixed}, nil
	case "4h":
		return intervalSpec{label: label, d: 4 * time.Hour, kind: intervalKindFixed}, nil
	case "6h":
		return intervalSpec{label: label, d: 6 * time.Hour, kind: intervalKindFixed}, nil
	case "12h":
		return intervalSpec{label: label, d: 12 * time.Hour, kind: intervalKindFixed}, nil
	case "1d":
		return intervalSpec{label: label, d: 24 * time.Hour, kind: intervalKindFixed}, nil
	case "3d":
		return intervalSpec{label: label, d: 3 * 24 * time.Hour, kind: intervalKindFixed}, nil
	case "1w":
		return intervalSpec{label: label, d: 7 * 24 * time.Hour, kind: intervalKindWeekly}, nil
	default:
		return intervalSpec{}, fmt.Errorf("candle builder: unknown interval %q", label)
	}
}

func (b *CandleBuilder) shardForSymbol(symbol string) *symbolShard {
	var h uint32 = 2166136261
	for i := 0; i < len(symbol); i++ {
		h ^= uint32(symbol[i])
		h *= 16777619
	}
	return &b.shards[h%uint32(len(b.shards))]
}
