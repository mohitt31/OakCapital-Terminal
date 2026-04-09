# DB Writer Integration Guide

## Overview

The `internal/dbwriter` package provides **durable persistence** for all events flowing through your event bus. It consumes messages from Redis Streams and writes them to PostgreSQL with **at-least-once delivery** semantics.

## Architecture

```
Redis Streams                          DBWriter Consumers                    PostgreSQL
┌─────────────────────────────┐       ┌──────────────────────┐             ┌───────────────┐
│ events:trades:stream:RELIANCE├──────→ TradeWriter          ├────────────→│ trades        │
│                              │       │ conns:db-writer      │             │               │
└─────────────────────────────┘       └──────────────────────┘             └───────────────┘

┌─────────────────────────────┐       ┌──────────────────────┐             ┌───────────────┐
│ events:orders:stream        ├──────→ OrderWriter          ├────────────→│ order_history │
│                              │       │ group:db-writer      │             │               │
└─────────────────────────────┘       └──────────────────────┘             └───────────────┘

┌─────────────────────────────┐       ┌──────────────────────┐             ┌───────────────┐
│ events:error:stream         ├──────→ ErrorWriter          ├────────────→│ system_event_ │
│                              │       │ single consumer      │             │ log           │
└─────────────────────────────┘       └──────────────────────┘             └───────────────┘

┌─────────────────────────────┐       ┌──────────────────────┐             ┌───────────────┐
│ events:candle:stream:*:1s   ├──────→ CandleWriter         ├────────────→│ candles       │
│ (batched per symbol)         │       │ (60-candle batches)  │             │               │
└─────────────────────────────┘       └──────────────────────┘             └───────────────┘
```

## Key Concepts

### 1. **At-Least-Once Delivery**
- Messages are **only acknowledged (ACK'd) after successful database insert**
- If your app crashes before ACK, the message stays in Redis PEL (Pending Entry List)
- Next startup retry: consumer auto-resumes from where it left off
- **Result**: No trades/orders lost, but possible duplicates (handle with UNIQUE constraints)

### 2. **Consumer Groups**
- All writers use Redis consumer groups for coordinated, resumable consumption
- Group name: `"db-writer"` (can scale horizontally with multiple nodes)
- Consumer name: `"dbw-node-0"` (configurable for multi-node setups)

### 3. **Batch Efficiency**
- **CandleWriter**: Batches up to 60 candles for bulk insert (pgx.CopyFrom)
- **TradeWriter**: Single-message per trade (small, individual inserts)
- **OrderWriter**: Single-message per order update
- **ErrorWriter**: Single-message per error
- Batching reduces round-trip times and database load

### 4. **Error Handling**
- DB write fails? Message is **not ACK'd**, stays in PEL, retried next startup
- Transient DB errors (network timeout) are automatically retried
- Permanent errors (schema mismatch, FK constraint) need manual investigation

## Integration in main.go

### Basic Integration

Add this to your `cmd/server/main.go` after PostgreSQL and Redis are initialized:

```go
package main

import (
    "synthbull/internal/dbwriter"
    // ... other imports
)

func main() {
    // ... existing code ...

    // After initializing db.Pool and bus (event bus):

    // Build symbol→id map
    symbolMap := make(map[string]int64)
    // Populate from your instrument seeding

    // Start all DB writers
    dbWriterSvc := dbwriter.StartDBWriters(
        ctx,
        bus.Sub,          // Redis subscriber
        db.Pool,          // PostgreSQL connection pool
        symbolMap,        // symbol → instrument_id mapping
        []string{"RELIANCE", "TCS", "INFY", ...},  // all trading symbols
    )
    defer dbWriterSvc.Stop()

    // Continue with rest of server startup...
}
```

### Advanced: Per-Writer Control

For more granular control, start individual writers:

```go
// Create the service
dbWriterSvc := dbwriter.NewService(bus.Sub, db.Pool, symbolMap)

// Start only specific writers
dbWriterSvc.StartTradeWriter(ctx, allSymbols)
dbWriterSvc.StartOrderWriter(ctx)
dbWriterSvc.StartErrorWriter(ctx)

// Or start all at once
dbWriterSvc.Start(ctx, allSymbols)

defer dbWriterSvc.Stop()
```

### Access Repositories Directly

For one-off operations or backfills:

```go
repos := dbWriterSvc.GetRepositories()

// Insert a single trade directly
err := repos.Trade.InsertTrade(ctx, myTradeEvent)

// Batch insert historical trades
err := repos.Trade.BatchTradeInsert(ctx, historicalTrades)
```

## Writers Explained

### CandleWriter

Batches closed candlestick events and inserts them in bulk.

**Event Source**: `events:candle:stream:{symbol}:{interval}` (Redis stream)

**Operation**:
1. Consumes up to 60 closed candles
2. Calls `onBatch(batch)` handler
3. Handler inserts to `candles` table via pgx.CopyFrom
4. On success, ACKs all 60 messages
5. On failure, keeps in PEL for retry

**Handler Example**:
```go
cw.Start(ctx, allSymbols, []string{"1s", "5s"}, func(batch []eventbus.CandleEvent) error {
    // Recommended: use pgx.CopyFrom for bulk inserts
    return candleRepo.InsertBatch(ctx, batch)
})
```

### TradeWriter

Persists every executed trade to the database.

**Event Source**: `events:trades:stream:{symbol}` (per-symbol Redis stream)

**Operation**:
1. One consumer goroutine per symbol
2. For each TradeEvent:
   - Resolve symbol → instrument_id from symbolMap
   - Convert prices (cents → decimal)
   - INSERT to `trades` table
3. ACK on success, retry on failure

**Handler Example**:
```go
tw.Start(ctx, allSymbols, func(trade eventbus.TradeEvent) error {
    // Insert single trade
    _, err := db.Pool.Exec(ctx, `
        INSERT INTO trades (...) VALUES ($1, $2, ...)
    `, ...)
    return err
})
```

### OrderWriter

Logs all order state transitions to an append-only history table.

**Event Source**: `events:orders:stream` (global Redis stream)

**Operation**:
1. Single global consumer group
2. Filters for FILLED/PARTIALLY_FILLED states only
3. INSERTs order snapshot to `order_history` table
4. Allows reconstructing full order lifecycle by joining on (order_id, updated_at)

**Handler Example**:
```go
ow.Start(ctx, func(orderUpdate eventbus.OrderUpdateEvent) error {
    // Only FILLED and PARTIALLY_FILLED reach here
    return db.InsertOrderHistory(ctx, orderUpdate)
})
```

### ErrorWriter

Captures all system errors for operational visibility.

**Event Source**: `events:error:stream` (global Redis stream)

**Operation**:
1. Consumes ErrorEvents from the event bus
2. Stores in `system_event_log` with severity/code/details
3. Useful for debugging, alerting, and compliance

### AlertWriter

User-facing notifications (price thresholds, portfolio milestones, order status).

**Status**: Currently requires manual per-user topic subscription.

**Future**: Implement global alerts stream for easier scaling.

### BotStatusWriter

Audit trail of automated bot activity.

**Status**: Similar to AlertWriter, requires per-bot subscription.

**Use Case**: Debugging bot behavior, P&L attribution, historical replay.

## Database Schema

All tables are created automatically via migration: `internal/db/migrations/sql/20_dbwriter_tables.sql`

### Trades Table

```sql
CREATE TABLE trades (
    id                        BIGSERIAL PRIMARY KEY,
    instrument_id             BIGINT NOT NULL,
    buy_order_id              BIGINT NOT NULL,
    sell_order_id             BIGINT NOT NULL,
    price                     DECIMAL(20,8) NOT NULL,
    quantity                  DECIMAL(20,8) NOT NULL,
    side_that_took_liquidity  VARCHAR(16) NOT NULL,
    executed_at               TIMESTAMPTZ NOT NULL,
    UNIQUE(instrument_id, buy_order_id, sell_order_id, executed_at)
);
```

### Order History Table

```sql
CREATE TABLE order_history (
    id              BIGSERIAL PRIMARY KEY,
    order_id        BIGINT NOT NULL,
    user_id         TEXT,
    symbol          VARCHAR(16) NOT NULL,
    side            VARCHAR(8) NOT NULL,
    order_type      VARCHAR(16) NOT NULL,
    status          VARCHAR(32) NOT NULL,
    quantity        DECIMAL(20,8) NOT NULL,
    filled_qty      DECIMAL(20,8) NOT NULL,
    avg_price       DECIMAL(20,8) NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL,
    UNIQUE(order_id, updated_at)
);
```

### System Event Log

```sql
CREATE TABLE system_event_log (
    id              BIGSERIAL PRIMARY KEY,
    event_type      VARCHAR(128) NOT NULL,
    severity        VARCHAR(16) NOT NULL,
    reference_id    BIGINT,
    payload         JSONB,
    created_at      TIMESTAMPTZ NOT NULL
);
```

See `20_dbwriter_tables.sql` for full schema.

## Error Handling & Monitoring

### Blocked Consumer?

If a consumer is stuck in PEL (Pending Entry List):

```bash
# Check consumer group status
redis-cli XINFO GROUPS events:trades:stream:RELIANCE

# Check pending entries
redis-cli XPENDING events:trades:stream:RELIANCE db-writer

# Manual ACK (last resort — only after fixing root cause)
redis-cli XACK events:trades:stream:RELIANCE db-writer <message-id>
```

### Duplicate Trades?

Use UNIQUE constraints:
- `UNIQUE(instrument_id, buy_order_id, sell_order_id, executed_at)`
- Handles retries gracefully with `ON CONFLICT DO NOTHING`

### Writer Logs

All writers log their progress:
```
[dbwriter] trade writer started: 7 symbols
[dbwriter] wrote 100 trades for RELIANCE
[dbwriter] order writer started
[dbwriter] error writer started
[dbwriter] all writers started successfully
```

Monitor these logs for health.

## Performance Tuning

### Connection Pool

Adjust `db.Pool` settings in `internal/db/postgres.go`:

```go
config := pgxpool.Config{
    MaxConns:    20,     // number of writer goroutines + headroom
    MinConns:    5,
    MaxConnLifetime: time.Hour,
}
```

### Batch Sizes

For CandleWriter, tune batch size based on candle frequency:

```go
// 60 candles = 1 minute of 1s candles
cw := dbwriter.NewCandleWriter(bus.Sub, 60)

// Or for 5s candles:
cw := dbwriter.NewCandleWriter(bus.Sub, 12)  // 12 × 5s = 1 minute
```

### Indices

Optimize for your query patterns:

```sql
-- Add more indices if querying by user:
CREATE INDEX idx_order_history_user_time ON order_history(user_id, updated_at DESC);

-- For portfolio analysis:
CREATE INDEX idx_portfolio_snap_user_time ON portfolio_snapshots(user_id, snapshot_at DESC);
```

## Testing

### Unit Tests

```go
func TestTradeRepository_InsertTrade(t *testing.T) {
    pool := testDB.Pool(t)
    repo := NewTradeRepository(pool, map[string]int64{"RELIANCE": 1})

    trade := eventbus.TradeEvent{
        Symbol: "RELIANCE",
        Price: 295000,      // cents
        Quantity: 10,
        MakerOrderID: 1,
        TakerOrderID: 2,
        TakerSide: "buy",
        ExecutedAt: time.Now(),
    }

    err := repo.InsertTrade(context.Background(), trade)
    assert.NoError(t, err)
}
```

### Integration Tests

```go
func TestDBWriter_EndToEnd(t *testing.T) {
    // 1. Setup: create test tables
    // 2. Publish test event to Redis
    // 3. Start DBWriter
    // 4. Poll database until record appears
    // 5. Verify correctness
    // 6. Verify ACK'd in Redis PEL
}
```

## Troubleshooting

### Writer Not Starting

**Problem**: `[dbwriter] skipping: Redis or PostgreSQL unavailable`

**Solution**: Ensure both are initialized before calling `StartDBWriters`:

```go
if db.Pool == nil || bus == nil {
    log.Fatal("Database or event bus not initialized")
}
```

### Duplicate Records Despite UNIQUE Constraint

**Problem**: `Violates unique constraint`

**Solution**: Check if messages are being retried due to ACK failures:

```bash
redis-cli XLEN events:trades:stream:RELIANCE  # Check stream length
redis-cli XINFO GROUPS events:trades:stream:RELIANCE  # Check consumer lag
```

### High Latency Between Event & Database

**Problem**: Events published to Redis but slow to appear in database

**Solutions**:
1. Increase batch size for CandleWriter
2. Check database connection pool size
3. Monitor PEL size: `redis-cli XPENDING <stream> <group>`

## Future Enhancements

- [ ] Global alerts stream (instead of per-user pub/sub)
- [ ] Global bot status stream (instead of per-bot pub/sub)
- [ ] Metrics/observability (Prometheus counters/histograms)
- [ ] Dead letter queue (DLQ) for unprocessable messages
- [ ] Async batch processing (debounce writes for high-frequency events)
- [ ] Multi-node distributed consumer groups (horizontal scaling)

## Summary

The DBWriter package ensures **durable persistence** of all trading events:

✅ **At-least-once delivery**: No data loss, resumable on failure
✅ **Parallel processing**: Per-symbol and per-service consumers
✅ **Efficient batching**: Bulk inserts for high-frequency candles
✅ **Error resilience**: Automatic retry on DB failures
✅ **Operational visibility**: Full audit trails and error logs

Integrate with a single function call in main.go, then let it run in the background!
