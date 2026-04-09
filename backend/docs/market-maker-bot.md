# Market Maker Bot — Guide

## Overview

The Market Maker Bot (MMB) is a built-in trading bot that provides liquidity by continuously placing limit orders on both sides of the order book. It profits from the bid-ask spread while managing inventory risk through dynamic pricing.

## Architecture

```
internal/bot/
├── strategy.go          ← Strategy interface (all bots implement this)
├── models.go            ← IncomingMessage, OutgoingOrder
├── config.go            ← BotConfig (tuneable params per instance)
├── exchange_client.go   ← WebSocket client to matching engine
├── portfolio.go         ← Position & PnL tracking
├── manager.go           ← BotManager: create/start/stop/list + event loop
└── builtin/
    ├── market_maker.go      ← MarketMakerStrategy
    └── market_maker_test.go ← Test suite (mock exchange + 7 tests)

cmd/mmbot/main.go        ← Standalone entry point
```

### Key Components

| Component | File | Role |
|-----------|------|------|
| **Strategy Interface** | `strategy.go` | Contract: `Name()`, `OnMarketData()`, `OnFill()`, `OnAck()` |
| **BotManager** | `manager.go` | Lifecycle management + generic event loop |
| **ExchangeClient** | `exchange_client.go` | WebSocket connect/send/recv with thread-safe writes |
| **Portfolio** | `portfolio.go` | Per-symbol position tracking, PnL calculation |
| **BotConfig** | `config.go` | All trading parameters in one struct |
| **MarketMakerStrategy** | `builtin/market_maker.go` | The actual MM logic |

## How the Bot Works

### Event Loop

```
Exchange ──WebSocket──▶ BotManager.runLoop()
                            │
                            ├── market_data ──▶ Strategy.OnMarketData() ──▶ []OutgoingOrder ──▶ Exchange
                            ├── fill        ──▶ Strategy.OnFill()       ──▶ updates portfolio
                            ├── ack         ──▶ Strategy.OnAck()        ──▶ updates order state
                            └── trade       ──▶ logged
```

### Market Making Logic

On each `market_data` tick:

1. **Compute mid-price**: `mid = (best_bid + best_ask) / 2`
2. **Dynamic spread**: widens as inventory grows → `spread × (1 + 0.1 × |position| / max_position)`
3. **Inventory skew**: shifts quotes away from the overweight side → `skew = 0.01 × position`
4. **Final quotes**: `bid = mid − spread/2 − skew`, `ask = mid + spread/2 + skew`
5. **Position limits**: blocks buys at `+50`, blocks sells at `−50`
6. **Risk pause**: if `|position| > 20`, only allows the side that reduces exposure
7. **Stale order cleanup**: cancels all open orders every 5 ticks

### Default Parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| `Spread` | 0.2 | Base bid-ask spread |
| `BaseOrderSize` | 5 | Quantity per order |
| `MaxPosition` | 50 | Hard position limit |
| `InventorySkewFactor` | 0.01 | Quote skew per unit of inventory |
| `InventoryRiskThreshold` | 20 | Threshold for risk-pause mode |
| `DynamicSpreadMultiplier` | 0.1 | Spread widening factor |
| `CancelInterval` | 5 | Cancel stale orders every N ticks |

## Running the Bot

### Prerequisites

- Go 1.22+
- A running matching engine with WebSocket at `ws://localhost:8000/ws`

### Start the bot

```bash
# From project root
go run ./cmd/mmbot/

# Or with a custom exchange URL
WS_URL=ws://exchange.example.com/ws go run ./cmd/mmbot/
```

### Build and run

```bash
go build -o mmbot ./cmd/mmbot/
./mmbot
```

## Running Tests

The test suite uses a **mock exchange** — an in-process WebSocket server that simulates the matching engine. No external dependencies needed.

### Run all MM bot tests

```bash
# From project root
go test ./internal/bot/builtin/ -v -count=1
```

### Run a specific test

```bash
go test ./internal/bot/builtin/ -v -run TestBotConnectsAndPlacesOrders
go test ./internal/bot/builtin/ -v -run TestMultiSymbol
go test ./internal/bot/builtin/ -v -run TestCancelOrders
```

### Test Descriptions

| Test | What it verifies |
|------|-----------------|
| `TestBotConnectsAndPlacesOrders` | Bot connects via WS, receives market data, places buy+sell limit orders, processes fills |
| `TestMultiSymbol` | Bot handles 3 symbols (AAPL, GOOG, TSLA) concurrently with independent state |
| `TestCancelOrders` | Stale orders are cancelled every `CancelInterval` (5) ticks |
| `TestInventoryRiskPause` | When inventory exceeds risk threshold, bot only trades to reduce position |
| `TestPositionLimits` | At max long position, bot only sells (no new buys) |
| `TestPortfolioAccountingUnit` | Portfolio math: position tracking, avg price, realized PnL, unrealized PnL |
| `TestStrategyQuotes` | Quote computation: spread placement, inventory skew direction |
| `TestTradeMessageHandling` | Trade broadcast messages are handled without crashing |

### How the Mock Exchange Works

The test suite spins up `httptest.Server` with a WebSocket handler that:

1. Accepts a bot connection
2. Receives orders from the bot
3. For **limit orders**: sends an `ack` (accepted), then a **partial fill** (50% of quantity)
4. For **cancel orders**: sends an `ack` (cancelled)
5. Test code pushes `market_data` messages at controlled intervals

This lets us exercise the full order lifecycle without a real matching engine.

# Market Maker Bot — Guide

## Overview

The Market Maker Bot (MMB) is a built-in trading bot that provides liquidity by continuously placing limit orders on both sides of the order book. It profits from the bid-ask spread while managing inventory risk through dynamic pricing.

## Architecture

```
internal/bot/
├── strategy.go          ← Strategy interface (all bots implement this)
├── models.go            ← IncomingMessage, OutgoingOrder
├── config.go            ← BotConfig (tuneable params per instance)
├── exchange_client.go   ← WebSocket client to matching engine
├── portfolio.go         ← Position & PnL tracking
├── manager.go           ← BotManager: create/start/stop/list + event loop
└── builtin/
    ├── market_maker.go      ← MarketMakerStrategy
    └── market_maker_test.go ← Test suite (mock exchange + 7 tests)

cmd/mmbot/main.go        ← Standalone entry point
```

### Key Components

| Component | File | Role |
|-----------|------|------|
| **Strategy Interface** | `strategy.go` | Contract: `Name()`, `OnMarketData()`, `OnFill()`, `OnAck()` |
| **BotManager** | `manager.go` | Lifecycle management + generic event loop |
| **ExchangeClient** | `exchange_client.go` | WebSocket connect/send/recv with thread-safe writes |
| **Portfolio** | `portfolio.go` | Per-symbol position tracking, PnL calculation |
| **BotConfig** | `config.go` | All trading parameters in one struct |
| **MarketMakerStrategy** | `builtin/market_maker.go` | The actual MM logic |

## How the Bot Works

### Event Loop

```
Exchange ──WebSocket──▶ BotManager.runLoop()
                            │
                            ├── market_data ──▶ Strategy.OnMarketData() ──▶ []OutgoingOrder ──▶ Exchange
                            ├── fill        ──▶ Strategy.OnFill()       ──▶ updates portfolio
                            ├── ack         ──▶ Strategy.OnAck()        ──▶ updates order state
                            └── trade       ──▶ logged
```