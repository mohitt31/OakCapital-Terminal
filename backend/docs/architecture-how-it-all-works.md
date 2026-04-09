# How Everything Fits Together

A complete walkthrough of every component in the system, how data flows between them, and what happens when a user places a trade.

## The Big Picture

```
                    ┌──────────────────────────────────┐
                    │         React Frontend            │
                    │  (charts, order book, portfolio)  │
                    └──────────┬───────────▲────────────┘
                               │           │
                          REST API    WebSocket
                          (Gin)       (live updates)
                               │           │
┌──────────────────────────────▼───────────┴────────────────────────────┐
│                          Go Backend (port 8080)                       │
│                                                                       │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────┐  ┌───────────────┐ │
│  │ Auth Service │  │ Order Handler│  │ Portfolio │  │ Bot Manager   │ │
│  │ (JWT+bcrypt) │  │ (REST API)   │  │ Manager   │  │ (MM strategy) │ │
│  └──────┬──────┘  └──────┬───────┘  └─────┬────┘  └───────┬───────┘ │
│         │                │                 │               │         │
│         │         ┌──────▼───────┐         │               │         │
│         │         │ BookManager  │◀────────┘───────────────┘         │
│         │         │ (per-symbol  │                                    │
│         │         │  engine+mutex)│                                   │
│         │         └──────┬───────┘                                    │
│         │                │                                            │
│         │    ┌───────────▼────────────┐                               │
│         │    │   C++ Matching Engine   │                              │
│         │    │   (via CGO — direct     │                              │
│         │    │    function calls)       │                             │
│         │    └───────────┬────────────┘                               │
│         │                │                                            │
│         │         Trade happens!                                      │
│         │                │                                            │
│         │    ┌───────────▼────────────┐                               │
│         │    │   onTrade callback      │                              │
│         │    │   (fires immediately)   │                              │
│         │    └───┬────────┬───────┬───┘                               │
│         │        │        │       │                                   │
│         │        ▼        ▼       ▼                                   │
│         │   WebSocket   Redis   Candle                                │
│         │     Hub      Event    Builder                               │
│         │  (broadcast   Bus     (OHLCV)                               │
│         │   to clients)                                               │
│         │                                                             │
│  ┌──────▼──────┐                                                      │
│  │  PostgreSQL  │  ← users, portfolios, positions, trades, migrations │
│  └─────────────┘                                                      │
│  ┌─────────────┐                                                      │
│  │    Redis     │  ← event streams, pub/sub, candle cache             │
│  └─────────────┘                                                      │
└───────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────┐
│                    GBM Market Simulator                               │
│  (background goroutine — feeds orders into the same engine)          │
│                                                                       │
│  Every 50ms per symbol:                                               │
│    1. GBM updates base price                                          │
│    2. Places random bids + asks into engine                           │
│    3. Occasionally places market orders (creates trades)              │
│    4. onTrade fires → same broadcast pipeline as user trades          │
└──────────────────────────────────────────────────────────────────────┘
```

## Startup Sequence

When you run `go run ./cmd/server/`, here's what happens in order:

```
1. Load config (.env)
       │
2. Connect to PostgreSQL (Supabase)
   └── Run SQL migrations (11 files, tracked in schema_migrations table)
       │
3. Connect to Redis
   └── Initialize event bus (publisher + subscriber)
       │
4. Start WebSocket hub
   └── Background goroutine: manages client connections, broadcasts events
       │
5. Create BookManager
   └── Will hold one C++ engine per symbol
       │
6. Create market simulator
   └── For each symbol (7 Indian stocks):
       ├── Get-or-create the engine from BookManager (shared!)
       ├── Wire onTrade callback → hub.BroadcastTrades + Redis publish
       └── Wire onBasePrice callback → hub.UpdateCandlePrice + depth refresh
       │
7. Start all GBM generators
   └── Each symbol now has a goroutine placing orders every 50ms
       │
8. Create portfolio manager
   └── In-memory cache + async flush to PostgreSQL every 2s
       │
9. Wire auth service (JWT + bcrypt + email)
       │
10. Register all Gin routes
       │
11. Start HTTP server on :8080
       │
12. Wait for SIGINT/SIGTERM → graceful shutdown
```

## The Three Order Sources

Every order — whether from a user, a bot, or the simulator — goes through the exact same path:

```
Source              │  How it submits         │  What happens
────────────────────┼─────────────────────────┼──────────────────────
GBM Simulator       │  generator calls        │  Same C++ engine,
(background)        │  engine.AddLimit()      │  same matching,
                    │  engine.Market()        │  same onTrade callback
────────────────────┼─────────────────────────┼──────────────────────
User (REST API)     │  POST /api/v1/order/*   │  Handler calls
                    │  → handler calls        │  bookManager.GetOrCreate()
                    │  engine.AddLimit()      │  then engine.AddLimit()
────────────────────┼─────────────────────────┼──────────────────────
Bot (Market Maker)  │  WebSocket connection   │  Bot receives market_data,
                    │  → sends order JSON     │  sends limit/cancel orders
                    │  → handler processes    │  back over WebSocket
```

The key insight: **the simulator and the user share the same engine handle per symbol**. This is set up in `main.go`:

```go
// BookManager creates the engine
managed := bookManager.GetOrCreate(sym)

// Market simulator uses the SAME engine (not a copy)
mktMgr.AddSymbolWithHandle(sym, preset.Class, preset.Config,
    managed.GetHandle(), managed.GetMu())
```

So when the simulator places a bid at $195.00 and a user places a sell at $195.00, they match against each other in the C++ engine. The order book is unified.

## What Happens When a User Places an Order

Let's trace a complete limit buy order from REST to frontend:

### Step 1: User sends REST request

```
POST /api/v1/order/limit/add
{
  "symbol": "RELIANCE",
  "order_id": 42,
  "side": "buy",
  "quantity": 10,
  "limit_price": 295000    ← price in cents ($2,950.00)
}
```

### Step 2: Order handler processes it

`internal/api/order_handler.go` → `AddLimit()`:

1. Validate input (symbol required, side valid, qty > 0, price > 0)
2. Get or create the engine for this symbol: `bookManager.GetOrCreate("RELIANCE")`
3. Lock the per-symbol mutex (thread safety for the C++ engine)
4. Call the C++ engine: `book.AddLimit(42, engine.SideBuy, 10, 295000)`

### Step 3: C++ engine matches

The C++ `Book::addLimitOrder()` runs:

- If best ask <= 295000 (our bid crosses the ask): **trade happens!**
  - The engine fills our buy against the best resting sell order
  - Returns `OrderResult{Trades: [{Price: 295000, Qty: 10, ...}], Changes: [...]}`
- If best ask > 295000: order rests in the book waiting for a seller

### Step 4: Results flow back to Go

The CGO bridge (`cgo_bridge.go`) converts the C struct to Go:

```go
OrderResult{
    Trades: []Trade{
        {Price: 295000, Qty: 10, MakerOrderID: 7, TakerOrderID: 42, TimestampUnixNano: ...},
    },
    Changes: []LevelChange{
        {Price: 295000, Volume: 0, Side: 0},  // ask level consumed
    },
}
```

### Step 5: Handler broadcasts via callbacks

The `onTrade` callback (wired in `main.go`) fires:

```
onTrade fires
    │
    ├──▶ hub.BroadcastTrades("RELIANCE", trades, side)
    │         └── sends to all WS clients subscribed to RELIANCE
    │
    ├──▶ hub.BroadcastTicker("RELIANCE", bestBid, bestAsk, lastPrice)
    │         └── updates the ticker bar in frontend
    │
    ├──▶ hub.BroadcastDepth("RELIANCE", depth)
    │         └── updates the order book display in frontend
    │
    └──▶ bus.Pub.PublishTrade(TradeEvent{...})
              └── writes to Redis stream: events:trades:stream:RELIANCE
```

### Step 6: Candle builder updates

The WebSocket hub has an internal candle builder. On every GBM tick and every trade:

```
hub.UpdateCandlePrice("RELIANCE", 295000, timestamp)
    │
    └──▶ CandleBuilder.ProcessTick()
         ├── Update 1s candle: {Open: 294800, High: 295100, Low: 294500, Close: 295000, Volume: 150}
         └── Update 5s candle: same but for 5-second window
              │
              └── If candle period ended → broadcast completed candle to WS clients
```

### Step 7: Frontend receives WebSocket messages

The React frontend receives 3-4 messages within milliseconds:

```json
{"type": "trade",     "symbol": "RELIANCE", "data": {"price": 295000, "quantity": 10, ...}}
{"type": "ticker",    "symbol": "RELIANCE", "data": {"best_bid": 294900, "best_ask": 295100, ...}}
{"type": "orderbook", "symbol": "RELIANCE", "data": {"bids": [...], "asks": [...]}}
{"type": "candle",    "symbol": "RELIANCE", "data": {"open": 294800, "high": 295100, ...}}
```

Frontend updates:
- Candlestick chart → new candle bar or updated current bar
- Order book display → bid/ask levels refreshed
- Trade history → new row in the trade list
- Ticker bar → latest price, bid, ask

### Step 8: Portfolio updates

When `ApplyFill` is called on the portfolio manager:

```
ApplyFill(userID, "RELIANCE", "buy", 2950.00, 10, isLimit=true)
    │
    ├── Release $29,500 from blocked → used (was escrowed when order placed)
    ├── Deduct $29,500 from total cash
    ├── Add RELIANCE position: qty=10, avgEntry=2950.00
    ├── Calculate unrealized P&L
    └── Schedule async flush to PostgreSQL (batched every 2 seconds)
```

## The Event Bus (Redis)

The event bus uses two Redis patterns for different needs:

### Redis Streams (Durable)

For data that **must not be lost** — trades, order updates, errors:

```
events:trades:stream:RELIANCE     ← every trade, ordered, replayable
events:orders:stream              ← all order state changes
events:candle:stream:RELIANCE:1m  ← completed candles (closed)
events:error:stream               ← system errors for debugging
```

Streams use consumer groups — if a consumer crashes, it picks up where it left off. Like a lightweight Kafka.

### Redis Pub/Sub (Ephemeral)

For **real-time** data where missing one message is fine:

```
events:depth:pubsub:RELIANCE      ← order book changes (latest wins)
events:ticker:pubsub:RELIANCE     ← best bid/ask updates
events:candle:pubsub:RELIANCE:1s  ← live candle updates
events:portfolio:pubsub:user123   ← user's portfolio changes
events:orders:pubsub:user123      ← user's order updates
events:bot:pubsub:bot456          ← bot status changes
```

If a WebSocket client disconnects and reconnects, it doesn't need the old depth updates — just the latest snapshot.

### Who publishes what

```
C++ Engine (via onTrade) ──▶ PublishTrade(), PublishTicker()
Candle Builder ────────────▶ PublishCandle()
Portfolio Manager ─────────▶ PublishPortfolio()
Order Handler ─────────────▶ PublishOrderUpdate()
Bot Manager ───────────────▶ PublishBotStatus()
```

### Who subscribes to what

```
WebSocket Hub ◀── OnDepth(), OnTicker(), OnCandle() → forwards to browser clients
Candle Builder ◀── ConsumeTradesGroup() → aggregates into OHLCV
Portfolio Manager ◀── (called directly by order handler, not via event bus)
```

## The Candle Builder

Converts raw trades into OHLCV candlestick data that the frontend charts display.

```
Trade: {symbol: "RELIANCE", price: 295000, qty: 10, time: 14:30:01.234}
Trade: {symbol: "RELIANCE", price: 295200, qty: 5,  time: 14:30:01.567}
Trade: {symbol: "RELIANCE", price: 294800, qty: 8,  time: 14:30:01.890}
                    │
                    ▼
            ┌───────────────┐
            │ Candle Builder │
            └───────┬───────┘
                    │
          ┌─────────┼─────────┐
          ▼         ▼         ▼
      1s candle  1m candle  5m candle

1s candle for 14:30:01:
  Open:   295000  (first trade in this second)
  High:   295200  (highest price)
  Low:    294800  (lowest price)
  Close:  294800  (last trade)
  Volume: 23      (10 + 5 + 8)
```

There are actually **two** candle builders:

1. **In `internal/candle/builder.go`** — subscribes to Redis trade streams, builds candles for 1s/1m/5m, publishes completed candles back to Redis
2. **In `internal/eventbus/candle_builder.go`** — used inside the WebSocket hub, builds candles from GBM price ticks for live streaming to clients

Both exist because:
- The Redis-based one is durable and can be replayed
- The in-hub one is instant (no Redis round-trip) for the live WebSocket stream

## The Portfolio Manager

Tracks every user's money and positions in real-time.

```
                         Portfolio (per user)
                    ┌─────────────────────────────┐
                    │  Total Cash:    $97,050.00   │
                    │  Available:     $87,050.00   │ ← can place new orders with this
                    │  Blocked:       $10,000.00   │ ← escrowed for pending limit buys
                    │                               │
                    │  Positions:                   │
                    │    RELIANCE: +10 @ $2,950.00  │ ← long 10 shares
                    │    TCS:      -5  @ $3,800.00  │ ← short 5 shares
                    │                               │
                    │  Unrealized P&L: +$350.00     │ ← mark-to-market
                    │  Equity:        $97,400.00    │ ← cash + holdings value
                    └─────────────────────────────┘
```

### Cash escrow flow

```
1. User places BUY limit order: 10 shares @ $2,950
   AvailableCash: $100,000 → $70,500  (moved $29,500 to blocked)
   BlockedCash:   $0 → $29,500

2a. Order FILLS:
    BlockedCash: $29,500 → $0  (released from escrow)
    TotalCash: $100,000 → $70,500  (actually spent)
    Position: RELIANCE +10 @ $2,950

2b. Order CANCELLED:
    BlockedCash: $29,500 → $0
    AvailableCash: $70,500 → $100,000  (money returned)
```

### Persistence

The portfolio lives in **Go memory** for speed. Every 2 seconds, a background goroutine flushes dirty portfolios to PostgreSQL:

```
In-memory (fast reads/writes)
    │
    │  every 2 seconds, batch flush
    ▼
PostgreSQL (durable backup)
    portfolios table: user_id, total_cash, available_cash, blocked_cash
    positions table:  portfolio_id, instrument_id, net_quantity, avg_entry_price
```

If the server crashes, it reloads from PostgreSQL on next startup.

## The WebSocket Hub

Manages all live connections from browser clients.

```
Browser ──connect──▶ /ws
                     │
                     ▼
              ┌─────────────┐
              │  WS Handler  │
              │  (upgrade    │
              │   HTTP → WS) │
              └──────┬──────┘
                     │
              ┌──────▼──────┐
              │     Hub      │
              │              │
              │  clients map │
              │  broadcast   │──▶ fan-out to subscribed clients
              │  channel     │
              └──────────────┘
```

### Subscription model

A client sends a JSON message to subscribe:

```json
{"action": "subscribe", "symbols": ["RELIANCE", "TCS"], "events": ["trade", "candle", "orderbook"]}
```

The hub only sends events matching the client's subscriptions. A client watching RELIANCE won't get TCS events.

### Event types sent to clients

| Type | When | Data |
|---|---|---|
| `trade` | Every trade execution | price, qty, maker/taker IDs, side |
| `orderbook` | Order book changes | full bids[] + asks[] arrays |
| `ticker` | Best bid/ask changes | bestBid, bestAsk, lastPrice |
| `candle` | New/updated candle | OHLCV for 1s or 5s interval |
| `depth` | Depth update | price levels with volumes |

## The Market Maker Bot

An automated trader that provides liquidity.

```
Exchange (WS) ──market_data──▶ MarketMakerStrategy
                                      │
                               ┌──────┴──────┐
                               │  Compute:    │
                               │  mid price   │
                               │  + spread    │
                               │  + skew      │
                               │  + risk      │
                               └──────┬──────┘
                                      │
                               ┌──────▼──────┐
                               │ Place orders │
                               │ bid = mid-   │
                               │   spread/2   │
                               │ ask = mid+   │
                               │   spread/2   │
                               └──────┬──────┘
                                      │
                               ◀──limit orders──
```

The bot:
- Places bids slightly below mid-price and asks slightly above
- Widens spread when carrying more inventory (risk management)
- Skews quotes away from the side it's overweight on
- Pauses buying when position > threshold (risk pause)
- Cancels stale orders every 5 ticks

## How the GBM Simulator Creates a Realistic Market

Without the simulator, the order book is empty and nothing happens. The simulator:

1. **Every 50ms**, steps the GBM model to get a new base price:
   ```
   basePrice = basePrice × exp((μ - σ²/2)×dt + σ×√dt×Z)
   ```

2. **Places 0-5 bids** below the base price and **0-5 asks** above it — this fills the order book with liquidity

3. **15% of ticks**, places a **market order** — this crosses the spread and creates an actual trade

4. When a trade happens, the **onTrade callback** fires, which broadcasts to all WebSocket clients

The result: the frontend sees a live, moving candlestick chart with realistic price action, even with zero human users connected.

## Thread Safety Model

The C++ engine is **not thread-safe**. Here's how we handle concurrent access:

```
Per-symbol mutex (in BookManager's ManagedBook):
    │
    ├── GBM generator locks before placing orders
    ├── REST handler locks before processing user orders
    └── Bot locks before placing bot orders

All three use the SAME mutex for the SAME symbol.
Different symbols have independent mutexes — no contention.
```

This means: placing a RELIANCE order never blocks a TCS order, but two RELIANCE orders (one from user, one from simulator) are serialized.

## Shutdown Sequence

When the server receives SIGINT (Ctrl+C):

```
1. HTTP server stops accepting new connections (10s grace period)
2. GBM generators stop (no more simulated orders)
3. Bot manager stops all bots
4. Portfolio manager flushes all dirty portfolios to PostgreSQL
5. WebSocket hub disconnects all clients
6. BookManager closes all C++ engine handles (frees memory)
7. Redis event bus closes
8. PostgreSQL connection pool closes
9. Process exits
```

All deferred in reverse order of initialization. No data loss — portfolios are flushed before DB closes.
