# OakCapital — Backend

Go backend + C++ Matching Engine for the OakCapital trading platform.

## What This Does

This service is the core of the platform. It:

- **Matches orders** via a C++ Limit Order Book (AVL trees + FIFO queues, 1.4M+ orders/sec) called from Go over CGO
- **Simulates the market** using a Geometric Brownian Motion goroutine that continuously feeds synthetic orders into the engine
- **Serves the REST API** (Gin) for order placement, portfolio queries, bot management, and auth
- **Streams real-time data** to the React frontend via WebSocket — order book deltas, trade executions, OHLCV candles, portfolio updates
- **Runs trading bots** — Alpha Bot (EMA crossover), Market Maker Bot (spread capture), and Bulbul BYOB (visual node graph strategy)
- **Manages portfolios** in-memory with async PostgreSQL flush every 2 seconds
- **Persists everything** to PostgreSQL (users, orders, trades, candles, bot configs) and caches live data in Redis

## Structure

```
cmd/
  server/main.go          ← entry point — wires all services, starts HTTP server
  mmbot/main.go           ← standalone market maker bot runner

internal/
  engine/                 ← CGO bridge to C++ matching engine
  market/                 ← GBM market simulator + multi-symbol manager
  api/                    ← REST handlers (orders, portfolio, bots, auth, symbols)
  api/ws/                 ← WebSocket hub — fan-out to connected clients
  auth/                   ← JWT middleware, bcrypt, register/login
  bot/                    ← bot framework, market maker strategy
  simbot/                 ← Alpha Bot, Bulbul BYOB node graph evaluator
  portfolio/              ← in-memory P&L engine with cash escrow
  candle/                 ← OHLCV candle builder (1s, 5s intervals)
  eventbus/               ← Redis Streams + Pub/Sub event bus
  db/                     ← PostgreSQL pool, Redis client, migrations
  dbwriter/               ← durable event-to-DB writer (Redis → Postgres)

Matching-Engine/
  Limit_Order_Book/       ← C++ Book, Limit, Order classes (AVL + FIFO)
  src/engine_c_api.cpp    ← C ABI wrapper (extern "C") for CGO
  include/engine_c_api.h  ← exported C API header
  tests/                  ← smoke tests for the C++ engine
```

## Running

```bash
# From the root opensoft/ directory:
docker compose up --build

# Or run backend only:
cd Opensoft-26-Backend
docker compose up --build
```

Backend API available at `http://localhost:8976/api/v1`

WebSocket at `ws://localhost:8976/ws?token=<jwt>`

## Key Design Decisions

- **CGO (zero IPC)** — C++ engine is a shared library linked directly into the Go binary. No TCP, no serialisation on the hot path.
- **Symbol-level mutex sharding** — each trading symbol has its own engine instance and mutex, so symbols execute in parallel with no cross-symbol blocking.
- **In-memory portfolio** — all cash and holdings live in a Go map for zero-latency settlement; a background worker flushes to Postgres every 2 seconds.
- **Delta events** — the CGO bridge emits only the price levels that changed (at most 4 per order) rather than full book snapshots, keeping WebSocket broadcast volume low.
- **Dual Redis strategy** — Redis Streams for durable order/trade events; Redis Pub/Sub for high-frequency ephemeral ticks where occasional loss is acceptable.
