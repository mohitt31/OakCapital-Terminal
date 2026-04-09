// Synthetic market generator — runs a goroutine that produces a moving price via
// Geometric Brownian Motion (GBM) and continuously submits random limit orders around
// it to simulate real market liquidity (50-100 orders/sec). Occasionally submits
// market orders to create actual trades. Uses the C++ matching engine via CGO.
package market

import (
	"log"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"synthbull/internal/engine"
)

// Config holds GBM and order generation parameters.
type Config struct {
	InitialPrice float64 // S₀ — starting price (e.g. 100.0 means $100)
	Mu           float64 // μ — drift (annualized)
	Sigma        float64 // σ — volatility (annualized)

	TickIntervalMS   int     // how often to generate orders (ms)
	TickSize         float64 // minimum price increment (e.g. 0.05)
	MinSpread        float64 // minimum bid-ask spread
	MaxSpread        float64 // maximum bid-ask spread
	MinQty           int     // minimum order quantity
	MaxQty           int     // maximum order quantity
	MaxOrdersPerSide int     // max orders to place per side per tick
	AggressiveRate   float64 // probability (0-1) of submitting a market order per tick
}

// DefaultConfig returns production defaults matching the competition spec.
func DefaultConfig() Config {
	return Config{
		InitialPrice:     100.0,
		Mu:               0.1,
		Sigma:            0.3,
		TickIntervalMS:   50, // 20 ticks/sec → up to 200 orders/sec at 5+5 per tick
		TickSize:         0.05,
		MinSpread:        0.10,
		MaxSpread:        0.50,
		MinQty:           1,
		MaxQty:           10,
		MaxOrdersPerSide: 5,
		AggressiveRate:   0.15, // 15% of ticks also submit a market order
	}
}

// Generator produces synthetic market data using GBM and feeds orders
// directly into the matching engine. It runs as a background goroutine.
type Generator struct {
	cfg    Config
	book   *engine.Handle
	mu     *sync.Mutex // protects concurrent access to the engine
	stopCh chan struct{}
	wg     sync.WaitGroup
	runMu  sync.Mutex
	active bool

	nextOrderID atomic.Int64
	rng         *rand.Rand

	symbol string // set via SetSymbol for event payloads

	// Optional callback for when trades occur
	onTrade func(side int, result engine.OrderResult)

	emitMu sync.RWMutex
	// Optional hooks for event-bus integration (set before Start; keep handlers fast).
	onBasePrice      func(BasePriceSample)
	onGBMReferenceBk func(GBMReferenceBook)
}

// New creates a generator that will submit orders to the given engine handle.
// The mutex must be the same one used by all other code touching this book.
func New(cfg Config, book *engine.Handle, bookMu *sync.Mutex) *Generator {
	return &Generator{
		cfg:    cfg,
		book:   book,
		mu:     bookMu,
		stopCh: make(chan struct{}),
		// Each symbol's Generator has its own RNG → independent Z shocks vs other books.
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetStartOrderID sets the starting order ID counter.
// Call before Start() to avoid ID collisions with other order sources.
func (g *Generator) SetStartOrderID(start int64) {
	g.nextOrderID.Store(start)
}

// SetOnTrade sets a callback function that will be called when trades occur.
// The callback receives the taker side and the order result containing trades.
func (g *Generator) SetOnTrade(callback func(side int, result engine.OrderResult)) {
	g.onTrade = callback
}

// SetSymbol sets the ticker name on emitted events (call before Start).
func (g *Generator) SetSymbol(symbol string) {
	g.symbol = symbol
}

// SetOnBasePrice registers a callback invoked every tick after the GBM step with latent S_t.
// Use for a candle-builder / price channel on your event bus.
func (g *Generator) SetOnBasePrice(fn func(BasePriceSample)) {
	g.emitMu.Lock()
	defer g.emitMu.Unlock()
	g.onBasePrice = fn
}

// SetOnGBMReferenceBook registers a callback with synthetic bid/ask around the GBM mid
// (untracked liquidity reference). Fan this into the same bus topic as MM “tracked” updates
// using Source / JSON field discrimination.
func (g *Generator) SetOnGBMReferenceBook(fn func(GBMReferenceBook)) {
	g.emitMu.Lock()
	defer g.emitMu.Unlock()
	g.onGBMReferenceBk = fn
}

// GetConfig returns the generator's configuration.
func (g *Generator) GetConfig() Config {
	return g.cfg
}

func (g *Generator) emitGBMEvents(basePrice float64) {
	ts := time.Now().UnixMilli()
	midCents := g.priceToInt(basePrice)
	// Symmetric reference spread using minimum spread (matches order placement geometry).
	half := g.cfg.MinSpread / 2.0
	bidF := g.roundDownToTick(basePrice - half)
	askF := g.roundUpToTick(basePrice + half)
	bidCents := g.priceToInt(bidF)
	askCents := g.priceToInt(askF)

	g.emitMu.RLock()
	cb, rb, sym := g.onBasePrice, g.onGBMReferenceBk, g.symbol
	g.emitMu.RUnlock()

	if cb != nil {
		cb(BasePriceSample{Symbol: sym, Timestamp: ts, Price: basePrice})
	}
	if rb != nil {
		rb(GBMReferenceBook{
			Symbol:       sym,
			Timestamp:    ts,
			Source:       SourceGBMUntracked,
			BestBidCents: bidCents,
			BestAskCents: askCents,
			MidCents:     midCents,
		})
	}
}

// Start begins the market simulation in a background goroutine.
func (g *Generator) Start() {
	g.runMu.Lock()
	defer g.runMu.Unlock()

	if g.active {
		return
	}
	g.stopCh = make(chan struct{})
	g.active = true
	g.wg.Add(1)
	go g.run()
	log.Printf("[market] generator started (interval=%dms, price=%.2f, μ=%.2f, σ=%.2f)",
		g.cfg.TickIntervalMS, g.cfg.InitialPrice, g.cfg.Mu, g.cfg.Sigma)
}

// Stop halts the generator. Safe to call multiple times.
func (g *Generator) Stop() {
	g.runMu.Lock()
	if !g.active {
		g.runMu.Unlock()
		return
	}
	close(g.stopCh)
	g.active = false
	g.runMu.Unlock()

	g.wg.Wait()
	log.Println("[market] generator stopped")
}

// tradingSecondsPerYear is used to scale GBM drift/volatility from
// annualized params down to per-tick increments.
const tradingSecondsPerYear = 252.0 * 6.5 * 3600.0

func (g *Generator) run() {
	defer g.wg.Done()

	ticker := time.NewTicker(time.Duration(g.cfg.TickIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	basePrice := g.cfg.InitialPrice
	dt := (float64(g.cfg.TickIntervalMS) / 1000.0) / tradingSecondsPerYear

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			// GBM step: S_t = S_{t-1} * exp((μ - σ²/2)*dt + σ*√dt*Z)
			z := g.rng.NormFloat64()
			basePrice = basePrice * math.Exp(
				(g.cfg.Mu-0.5*g.cfg.Sigma*g.cfg.Sigma)*dt+
					g.cfg.Sigma*math.Sqrt(dt)*z,
			)

			g.emitGBMEvents(basePrice)
			g.submitOrders(basePrice)
		}
	}
}

// priceToInt converts a float price to integer cents.
// The engine works in integer prices to avoid floating-point issues.
func (g *Generator) priceToInt(price float64) int {
	return int(math.Round(price * 100))
}

// roundDownToTick rounds a price down to the nearest tick.
func (g *Generator) roundDownToTick(price float64) float64 {
	return math.Floor(price/g.cfg.TickSize) * g.cfg.TickSize
}

// roundUpToTick rounds a price up to the nearest tick.
func (g *Generator) roundUpToTick(price float64) float64 {
	return math.Ceil(price/g.cfg.TickSize) * g.cfg.TickSize
}

func (g *Generator) submitOrders(basePrice float64) {
	bidCount := g.rng.Intn(g.cfg.MaxOrdersPerSide + 1)
	askCount := g.rng.Intn(g.cfg.MaxOrdersPerSide + 1)

	g.mu.Lock()
	defer g.mu.Unlock()

	// Submit bids (buy orders below base price)
	for i := 0; i < bidCount; i++ {
		spread := g.cfg.MinSpread + g.rng.Float64()*(g.cfg.MaxSpread-g.cfg.MinSpread)
		bidPrice := g.roundDownToTick(basePrice - spread/2.0)
		bidPriceCents := g.priceToInt(bidPrice)
		qty := g.cfg.MinQty + g.rng.Intn(g.cfg.MaxQty-g.cfg.MinQty+1)
		orderID := int(g.nextOrderID.Add(1))

		if bidPriceCents <= 0 {
			continue
		}

		status, result := g.book.AddLimit(orderID, engine.SideBuy, qty, bidPriceCents)
		if status != engine.StatusOK {
			log.Printf("[market] bid failed: order=%d price=%d qty=%d err=%s",
				orderID, bidPriceCents, qty, status.Error())
			continue
		}

		if len(result.Trades) > 0 && g.onTrade != nil {
			g.onTrade(engine.SideBuy, result)
		}
	}

	// Submit asks (sell orders above base price)
	for i := 0; i < askCount; i++ {
		spread := g.cfg.MinSpread + g.rng.Float64()*(g.cfg.MaxSpread-g.cfg.MinSpread)
		askPrice := g.roundUpToTick(basePrice + spread/2.0)
		askPriceCents := g.priceToInt(askPrice)
		qty := g.cfg.MinQty + g.rng.Intn(g.cfg.MaxQty-g.cfg.MinQty+1)
		orderID := int(g.nextOrderID.Add(1))

		if askPriceCents <= 0 {
			continue
		}

		status, result := g.book.AddLimit(orderID, engine.SideSell, qty, askPriceCents)
		if status != engine.StatusOK {
			log.Printf("[market] ask failed: order=%d price=%d qty=%d err=%s",
				orderID, askPriceCents, qty, status.Error())
			continue
		}

		if len(result.Trades) > 0 && g.onTrade != nil {
			g.onTrade(engine.SideSell, result)
		}
	}

	// Occasionally submit a market order to simulate aggressive traders
	// taking liquidity. This creates actual trades in the book.
	// if g.cfg.AggressiveRate > 0 && g.rng.Float64() < g.cfg.AggressiveRate {
	// 	side := engine.SideBuy
	// 	if g.rng.Float64() < 0.5 {
	// 		side = engine.SideSell
	// 	}
	// 	qty := g.cfg.MinQty + g.rng.Intn(g.cfg.MaxQty-g.cfg.MinQty+1)
	// 	orderID := int(g.nextOrderID.Add(1))

	// 	status, result := g.book.Market(orderID, side, qty)
	// 	if status == engine.StatusOK && len(result.Trades) > 0 && g.onTrade != nil {
	// 		g.onTrade(side, result)
	// 	}
	// }
}
