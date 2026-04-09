# Synthetic Market Simulation

How the market generator produces realistic price movement and order flow without any external data.

**Source code**: `internal/market/generator.go`, `internal/market/manager.go`

## Overview

The generator runs as a background goroutine that:

1. Moves a **base price** using GBM (Geometric Brownian Motion) every tick
2. Submits **random limit orders** (bids + asks) around that base price into the C++ matching engine
3. Occasionally submits **market orders** to create actual trades

This creates a self-sustaining market — the order book fills with liquidity, prices move realistically, and trades happen continuously.

```
Every 50ms:
   ┌──────────────────────────────┐
   │  GBM step: update basePrice  │
   └──────────┬───────────────────┘
              │
   ┌──────────▼───────────────────┐
   │  Generate 0-5 bids below mid │──▶ engine.AddLimit(BUY)
   │  Generate 0-5 asks above mid │──▶ engine.AddLimit(SELL)
   └──────────┬───────────────────┘
              │
   ┌──────────▼───────────────────┐
   │  15% chance: market order    │──▶ engine.Market() → creates a trade
   └──────────────────────────────┘
```

## The GBM Formula

From the competition spec:

```
S_t = S₀ × exp((μ - σ²/2)t + σW_t)
```

In code, we apply this incrementally each tick:

```go
z := rng.NormFloat64()                    // W_t: standard normal random
basePrice = basePrice * math.Exp(
    (mu - 0.5*sigma*sigma) * dt           // drift component
    + sigma * math.Sqrt(dt) * z,          // volatility component
)
```

Where:
- `basePrice` is S_{t-1} (price from previous tick)
- `mu` = 0.1 (annualized drift — price tends to go up 10%/year)
- `sigma` = 0.3 (annualized volatility — 30% yearly price swings)
- `dt` = tick interval scaled to a fraction of a trading year
- `z` = random number from normal distribution (the "Brownian motion" part)

### What `dt` Actually Is

```go
dt = (tickIntervalMS / 1000.0) / tradingSecondsPerYear
```

`tradingSecondsPerYear = 252 × 6.5 × 3600 = 5,896,800 seconds`

This is the number of seconds in a trading year (252 trading days × 6.5 hours/day). We divide our tick interval by this to scale the annualized μ and σ down to per-tick values.

With default 50ms ticks: `dt ≈ 8.5 × 10⁻⁹`

This means per-tick price movement is tiny (fractions of a cent), but over thousands of ticks it produces realistic drift and volatility patterns.

### Why GBM?

GBM is the standard model for asset prices because:
- Prices can't go negative (multiplicative, not additive)
- Returns are log-normally distributed (matches real markets)
- It's the foundation of the Black-Scholes option pricing model
- Simple to implement, well-understood behavior

## Order Generation

On each tick, the generator places random limit orders around the current base price.

### Bid (Buy) Orders

```
bidPrice = roundDown(basePrice - spread/2, tickSize)
```

- Placed **below** the base price (buyers want to buy cheap)
- Rounded down to nearest tick (0.05) for realistic price levels
- Random spread between 0.10 and 0.50
- Random quantity between 1 and 10

### Ask (Sell) Orders

```
askPrice = roundUp(basePrice + spread/2, tickSize)
```

- Placed **above** the base price (sellers want to sell high)
- Rounded up to nearest tick
- Same random spread and quantity ranges

### How Many Orders Per Tick

Each tick generates `0 to MaxOrdersPerSide` bids and asks independently. With defaults:
- 0-5 bids + 0-5 asks per tick
- At 20 ticks/sec (50ms interval), that's roughly **50-100 orders/sec** (matching the spec requirement)

### Price Conversion

The GBM works in dollars (float64), but the C++ engine works in **integer cents** to avoid floating-point matching bugs:

```go
priceCents = int(math.Round(price * 100))
```

So `$99.95` becomes `9995` in the engine.

## Aggressive Market Orders

Without market orders, the book fills up with resting limit orders but no trades ever happen — bids are always below asks by design.

To create actual trades, the generator submits a **market order** on ~15% of ticks:

```go
if rand() < AggressiveRate {
    // randomly buy or sell
    engine.Market(orderID, randomSide, randomQty)
}
```

This simulates real traders who "take" liquidity by hitting the best bid/ask. It's what makes the price chart move and the trade history populate.

## All Parameters

| Parameter | Default | What it controls | Tweak for... |
|---|---|---|---|
| `InitialPrice` | 100.0 | Starting price in dollars | Different asset class |
| `Mu` | 0.1 | Annual drift (direction bias) | More bullish (↑) or bearish (↓) trend |
| `Sigma` | 0.3 | Annual volatility | Calmer (↓0.1) or wilder (↑0.5) price swings |
| `TickIntervalMS` | 50 | Milliseconds between order batches | Faster (↓10) = more orders/sec, slower (↑200) = less load |
| `TickSize` | 0.05 | Min price increment | Granularity of price levels in the book |
| `MinSpread` | 0.10 | Tightest bid-ask spread | Smaller = tighter market, more likely to trade |
| `MaxSpread` | 0.50 | Widest bid-ask spread | Larger = wider book depth |
| `MinQty` | 1 | Smallest order size | Trade size range |
| `MaxQty` | 10 | Largest order size | Trade size range |
| `MaxOrdersPerSide` | 5 | Max bids or asks per tick | Book depth per tick (total orders/sec ≈ this × 2 × 1000/interval) |
| `AggressiveRate` | 0.15 | % of ticks with a market order | Higher = more trades, lower = more passive book building |

## Common Tweaks

### "Price moves too slowly / chart looks flat"

Increase `Sigma` (volatility):
```go
cfg.Sigma = 0.5  // was 0.3 — 67% more volatile
```

Or increase `Mu` for a stronger directional trend:
```go
cfg.Mu = 0.3  // was 0.1 — stronger upward drift
```

### "Not enough trades happening"

Increase `AggressiveRate`:
```go
cfg.AggressiveRate = 0.30  // was 0.15 — market order on 30% of ticks
```

### "Order book looks thin / not enough depth"

Increase `MaxOrdersPerSide`:
```go
cfg.MaxOrdersPerSide = 10  // was 5 — twice as many orders per tick
```

Or decrease `TickIntervalMS` for more frequent order submission:
```go
cfg.TickIntervalMS = 20  // was 50 — 50 ticks/sec instead of 20
```

### "Spread is too wide on the frontend chart"

Tighten the spread range:
```go
cfg.MinSpread = 0.02  // was 0.10
cfg.MaxSpread = 0.10  // was 0.50
```

### "I want a crypto-like volatile market"

```go
cfg := market.Config{
    InitialPrice:     50000.0,   // BTC-like starting price
    Mu:               0.0,       // no directional bias
    Sigma:            0.8,       // very volatile
    TickIntervalMS:   20,        // fast updates
    TickSize:         1.0,       // $1 increments
    MinSpread:        5.0,       // $5 minimum spread
    MaxSpread:        50.0,      // $50 max spread
    MinQty:           1,
    MaxQty:           5,
    MaxOrdersPerSide: 8,
    AggressiveRate:   0.20,
}
```

### "I want a calm stock-like market"

```go
cfg := market.Config{
    InitialPrice:     150.0,     // AAPL-like
    Mu:               0.05,      // slight upward drift
    Sigma:            0.15,      // low volatility
    TickIntervalMS:   100,       // slower updates
    TickSize:         0.01,      // penny increments
    MinSpread:        0.01,      // very tight spread
    MaxSpread:        0.05,
    MinQty:           10,
    MaxQty:           100,       // larger lot sizes
    MaxOrdersPerSide: 5,
    AggressiveRate:   0.10,
}
```

## How to Use in Code

```go
import (
    "sync"
    "synthbull/internal/engine"
    "synthbull/internal/market"
)

// Create the matching engine (one per symbol)
book := engine.New()
defer book.Close()

// Shared mutex — ALL code that touches this book must use the same mutex
var bookMu sync.Mutex

// Create and start the generator
cfg := market.DefaultConfig()
gen := market.New(cfg, book, &bookMu)
gen.SetStartOrderID(1_000_000)  // reserve 0-999999 for user/bot orders
gen.Start()

// ... the market is now live, orders are flowing, trades are happening ...

// On shutdown:
gen.Stop()
```

## How to Run Tests

```bash
# Test the generator (creates real C++ engine, feeds real orders, verifies trades)
CGO_ENABLED=1 go test -v ./internal/market/

# Test everything (engine + generator)
CGO_ENABLED=1 go test -v ./internal/engine/ ./internal/market/
```

## Relationship to Other Components

```
                          ┌─────────────┐
                          │   Frontend   │
                          │  (React.js)  │
                          └──────▲───────┘
                                 │ WebSocket
                          ┌──────┴───────┐
                          │   API/WS Hub │ ◀── reads depth, trades
                          └──────▲───────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                         │
  ┌─────┴──────┐          ┌─────┴──────┐          ┌──────┴───────┐
  │  Generator  │          │  User API   │          │     Bots     │
  │   (GBM)     │          │  (REST)     │          │  (MM, Alpha) │
  └─────┬──────┘          └─────┬──────┘          └──────┬───────┘
        │                       │                         │
        │   AddLimit/Market     │   AddLimit/Market       │   AddLimit/Market
        └───────────┬───────────┴─────────────────────────┘
                    │
             ┌──────▼───────┐
             │  C++ Engine   │
             │  (via CGO)    │
             └──────────────┘
```

The generator is just one of three order sources. Users and bots also submit orders to the same engine. They all share the same mutex and the same C++ Book instance.

## Multi-Symbol Manager

The system supports multiple symbols across 3 asset classes. Each symbol gets its own independent C++ matching engine, GBM generator, and mutex.

**Source code**: `internal/market/manager.go`

### Asset Classes

| Class | Characteristics | Symbols |
|---|---|---|
| **Crypto** | High volatility, no drift, fast ticks (50ms), large prices | BTC, ETH, SOL |
| **Stock** | Moderate volatility, slight upward drift, penny tick sizes | AAPL, GOOGL, TSLA |
| **ETF** | Low volatility, tight spreads, high volume, slow ticks | SPY, QQQ, GLD |

### Default Symbols

| Symbol | Class | Start Price | Sigma | Tick Interval | Spread Range | Qty Range |
|---|---|---|---|---|---|---|
| BTC | crypto | $62,000 | 0.75 | 50ms | $10–$80 | 1–5 |
| ETH | crypto | $3,400 | 0.80 | 50ms | $2–$15 | 1–10 |
| SOL | crypto | $145 | 0.90 | 50ms | $0.20–$2 | 1–20 |
| AAPL | stock | $195 | 0.25 | 80ms | $0.02–$0.10 | 10–100 |
| GOOGL | stock | $175 | 0.28 | 80ms | $0.03–$0.15 | 5–80 |
| TSLA | stock | $250 | 0.50 | 60ms | $0.05–$0.30 | 5–50 |
| SPY | etf | $520 | 0.15 | 100ms | $0.01–$0.03 | 50–500 |
| QQQ | etf | $450 | 0.18 | 100ms | $0.01–$0.05 | 50–300 |
| GLD | etf | $215 | 0.12 | 120ms | $0.01–$0.04 | 20–200 |

### Why These Defaults?

- **Crypto** has zero drift (`Mu=0`) because crypto doesn't have a historical upward bias like stocks. High sigma (0.75–0.90) because crypto is volatile. Large tick sizes ($0.50–$1.00) and wide spreads match real crypto markets.
- **Stocks** have slight positive drift (`Mu=0.05–0.10`) to simulate the historical equity risk premium. Penny tick sizes and tight spreads match real stock exchanges. Larger lot sizes (10–100) because stock trading is higher volume per order.
- **ETFs** are the calmest — low sigma (0.12–0.18), very tight spreads ($0.01–$0.05), and high volume (50–500 qty). This matches real ETF markets like SPY where bid-ask spread is often just 1 cent.

### Usage

#### Quick start with all defaults

```go
import "synthbull/internal/market"

mgr := market.NewManager()
mgr.DefaultSymbols()   // registers all 9 symbols
mgr.StartAll()         // starts 9 engines + 9 generators

// ... market is live ...

mgr.StopAll()          // stops generators (books stay readable)
mgr.CloseAll()         // frees C++ memory
```

#### Add a custom symbol

```go
mgr := market.NewManager()

mgr.AddSymbol("DOGE", market.Crypto, market.Config{
    InitialPrice:     0.15,
    Mu:               0.0,
    Sigma:            1.2,       // very volatile meme coin
    TickIntervalMS:   30,
    TickSize:         0.0001,
    MinSpread:        0.001,
    MaxSpread:        0.005,
    MinQty:           1000,
    MaxQty:           50000,     // huge lot sizes
    MaxOrdersPerSide: 5,
    AggressiveRate:   0.25,
})

mgr.StartAll()
```

#### Access a specific symbol's book

```go
btc := mgr.GetSymbol("BTC")
if btc == nil {
    // symbol not registered
}

// Always lock the mutex before touching the book
btc.Mu.Lock()
depth := btc.Book.GetDepth()
btc.Mu.Unlock()

fmt.Printf("BTC: bestBid=%d bestAsk=%d\n", depth.BestBid, depth.BestAsk)
// Output: BTC: bestBid=6198200 bestAsk=6198500
// (prices in cents: $61,982.00 bid, $61,985.00 ask)
```

#### List symbols by asset class

```go
cryptos := mgr.ListByClass(market.Crypto)  // ["BTC", "ETH", "SOL"]
stocks := mgr.ListByClass(market.Stock)    // ["AAPL", "GOOGL", "TSLA"]
etfs := mgr.ListByClass(market.ETF)        // ["SPY", "QQQ", "GLD"]
all := mgr.ListSymbols()                   // all 9
```

#### Submit a user order to a specific symbol

```go
aapl := mgr.GetSymbol("AAPL")
aapl.Mu.Lock()
status, result := aapl.Book.AddLimit(orderID, engine.SideBuy, 50, 19500) // buy 50 @ $195.00
aapl.Mu.Unlock()
```

### How It Works Internally

Each symbol is fully independent:

```
          Manager
             │
    ┌────────┼────────┬────────────┐
    ▼        ▼        ▼            ▼        ...
  [BTC]    [ETH]    [AAPL]      [SPY]
   │        │        │            │
   ├ Book   ├ Book   ├ Book      ├ Book     ← separate C++ engine per symbol
   ├ Mutex  ├ Mutex  ├ Mutex     ├ Mutex    ← separate lock per symbol
   └ Gen    └ Gen    └ Gen       └ Gen      ← separate GBM goroutine per symbol
```

- **No shared state** between symbols — BTC orders never affect AAPL's book
- **Independent mutexes** — trading BTC doesn't block AAPL
- **Unique order ID ranges** — each symbol gets IDs 10M apart (BTC: 10M+, ETH: 20M+, etc.)
- **Independent GBM** — each symbol has its own price trajectory

### Adding/Removing Symbols at Runtime

Currently `AddSymbol` works before or after `StartAll`, but individual start/stop per symbol is not implemented. If you need dynamic symbol management, the `SymbolInfo` struct is public — you can call `info.Generator.Start()` and `info.Generator.Stop()` directly.

### Tweaking a Specific Symbol

To change a symbol's behavior, modify the config in `DefaultSymbols()` inside `manager.go`. For example, to make TSLA more volatile:

```go
// In manager.go → DefaultSymbols()
m.AddSymbol("TSLA", Stock, Config{
    ...
    Sigma: 0.80,        // was 0.50 — much more volatile
    AggressiveRate: 0.25, // was 0.15 — more trades
    ...
})
```
