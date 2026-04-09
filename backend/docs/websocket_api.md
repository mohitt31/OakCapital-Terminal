# WebSocket API Documentation

Real-time market data streaming via WebSocket for the Oak Capital trading platform.

---

## Connection

### Endpoint
```
ws://localhost:8080/ws
```

Or with TLS:
```
wss://your-domain.com/ws
```

### Connection Example (JavaScript)
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
    console.log('Connected to WebSocket');

    // Subscribe to BTC trades
    ws.send(JSON.stringify({
        action: 'subscribe',
        symbols: ['BTC'],
        event: 'trade'
    }));
};

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};

ws.onclose = () => {
    console.log('Disconnected from WebSocket');
};

ws.onerror = (error) => {
    console.error('WebSocket error:', error);
};
```

---

## Client Messages

Clients send JSON messages to subscribe/unsubscribe from market data streams.

### Subscribe
```json
{
    "action": "subscribe",
    "symbols": ["BTC", "ETH", "AAPL"],
    "event": "trade"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `action` | string | Must be `"subscribe"` |
| `symbols` | string[] | List of symbols to subscribe to. Use `"*"` for all symbols |
| `event` | string | Event type: `"trade"`, `"ticker"`, `"depth"`, `"candle"`, or `"all"` |

### Unsubscribe
```json
{
    "action": "unsubscribe",
    "symbols": ["BTC"],
    "event": "trade"
}
```

### Ping (Keep-Alive)
```json
{
    "action": "ping"
}
```

Response:
```json
{
    "type": "pong",
    "success": true,
    "data": {
        "timestamp": 1711612800000
    }
}
```

### Status (Check Subscriptions)
```json
{
    "action": "status"
}
```

Response:
```json
{
    "type": "status",
    "success": true,
    "data": {
        "subscriptions": {
            "BTC": ["trade", "ticker"],
            "ETH": ["trade"]
        },
        "timestamp": 1711612800000
    }
}
```

---

## Server Messages

### Event Types

| Event | Description |
|-------|-------------|
| `trade` | Individual trade executions |
| `ticker` | Best bid/ask and last price updates |
| `depth` | Full order book depth snapshot |
| `candle` | OHLCV candlestick data |

### Trade Event
Sent when a trade is executed.

```json
{
    "type": "trade",
    "symbol": "BTC",
    "timestamp": 1711612800000,
    "data": {
        "price": 6200000,
        "quantity": 5,
        "maker_order_id": 12345,
        "taker_order_id": 12346,
        "side": 1,
        "timestamp": 1711612800000
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `price` | int | Trade price in cents (62000.00 = 6200000) |
| `quantity` | int | Trade quantity |
| `maker_order_id` | int | Order ID of the resting order |
| `taker_order_id` | int | Order ID of the aggressive order |
| `side` | int | Taker side: 0=sell, 1=buy |
| `timestamp` | int | Unix timestamp in milliseconds |

### Ticker Event
Sent when best bid/ask prices change.

```json
{
    "type": "ticker",
    "symbol": "BTC",
    "timestamp": 1711612800000,
    "data": {
        "best_bid": 6199500,
        "best_ask": 6200500,
        "last_price": 6200000,
        "timestamp": 1711612800000
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `best_bid` | int | Best bid price in cents |
| `best_ask` | int | Best ask price in cents |
| `last_price` | int | Last trade price in cents |

### Depth Event
Full order book snapshot.

```json
{
    "type": "depth",
    "symbol": "BTC",
    "timestamp": 1711612800000,
    "data": {
        "bestBid": 6199500,
        "bestAsk": 6200500,
        "lastPrice": 6200000,
        "bids": [
            {"price": 6199500, "volume": 10},
            {"price": 6199000, "volume": 25},
            {"price": 6198500, "volume": 15}
        ],
        "asks": [
            {"price": 6200500, "volume": 8},
            {"price": 6201000, "volume": 20},
            {"price": 6201500, "volume": 12}
        ]
    }
}
```

### Candle Event
OHLCV candlestick data.

```json
{
    "type": "candle",
    "symbol": "BTC",
    "timestamp": 1711612800000,
    "data": {
        "open": 6195000,
        "high": 6205000,
        "low": 6190000,
        "close": 6200000,
        "volume": 150,
        "timestamp": 1711612800000,
        "interval": 1
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `open` | int | Opening price in cents |
| `high` | int | Highest price in cents |
| `low` | int | Lowest price in cents |
| `close` | int | Closing price in cents |
| `volume` | int | Total volume traded |
| `interval` | int | Candle interval in seconds (1 or 5) |

---

## System Messages

### Connected
Sent immediately after successful connection.

```json
{
    "type": "connected",
    "success": true,
    "message": "WebSocket connection established",
    "data": {
        "timestamp": 1711612800000
    }
}
```

### Success Response
Sent after successful subscribe/unsubscribe.

```json
{
    "type": "success",
    "success": true,
    "message": "subscribed",
    "data": {
        "symbols": ["BTC", "ETH"],
        "event": "trade"
    }
}
```

### Error Response
Sent when an error occurs.

```json
{
    "type": "error",
    "success": false,
    "message": "no symbols specified"
}
```

---

## Available Symbols

### Crypto
- `BTC` - Bitcoin (price ~$62,000)
- `ETH` - Ethereum (price ~$3,400)
- `SOL` - Solana (price ~$145)

### Stocks
- `AAPL` - Apple Inc. (price ~$195)
- `GOOGL` - Alphabet Inc. (price ~$175)
- `TSLA` - Tesla Inc. (price ~$250)

### ETFs
- `SPY` - S&P 500 ETF (price ~$520)
- `QQQ` - Nasdaq 100 ETF (price ~$450)
- `GLD` - Gold ETF (price ~$215)

---

## Integration with Backend

### Wiring Up in main.go

```go
package main

import (
    "log"
    "net/http"

    "synthbull/internal/api/ws"
)

func main() {
    // Create and start the WebSocket hub
    hub := ws.NewHub()
    go hub.Run()

    // Create the handler
    wsHandler := ws.NewHandler(hub)

    // Register the WebSocket endpoint
    http.HandleFunc("/ws", wsHandler.ServeWS)

    // Start server
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Broadcasting Market Data

From your order handler or market generator:

```go
// After a trade executes
hub.BroadcastTrade("BTC", trade, takerSide)

// After order book changes
hub.BroadcastDepth("BTC", book.GetDepth())

// Periodic ticker updates
hub.BroadcastTicker("BTC", bestBid, bestAsk, lastPrice)

// Candle updates (from candle builder)
hub.BroadcastCandle("BTC", ws.CandleData{
    Open:      6195000,
    High:      6205000,
    Low:       6190000,
    Close:     6200000,
    Volume:    150,
    Timestamp: time.Now().UnixMilli(),
    Interval:  1,
})
```

### Integration with Market Generator

Modify `internal/market/generator.go` to broadcast trades:

```go
func (g *Generator) submitOrders(basePrice float64) {
    // ... existing order submission code ...

    // After market order execution, broadcast trades
    if len(result.Trades) > 0 {
        for _, trade := range result.Trades {
            g.hub.BroadcastTrade(g.symbol, trade, side)
        }
    }
}
```

---

## Testing Instructions

### 1. Using wscat (Command Line)

Install wscat:
```bash
npm install -g wscat
```

Connect and test:
```bash
# Connect
wscat -c ws://localhost:8080/ws

# Subscribe to BTC trades
> {"action": "subscribe", "symbols": ["BTC"], "event": "trade"}

# Subscribe to all events for ETH
> {"action": "subscribe", "symbols": ["ETH"], "event": "all"}

# Check status
> {"action": "status"}

# Ping
> {"action": "ping"}

# Unsubscribe
> {"action": "unsubscribe", "symbols": ["BTC"], "event": "trade"}
```

### 2. Using Browser Console

```javascript
// Connect
let ws = new WebSocket('ws://localhost:8080/ws');

// Setup handlers
ws.onopen = () => console.log('Connected');
ws.onmessage = (e) => console.log('Received:', JSON.parse(e.data));
ws.onerror = (e) => console.error('Error:', e);
ws.onclose = () => console.log('Disconnected');

// Subscribe to trades
ws.send(JSON.stringify({action: 'subscribe', symbols: ['BTC'], event: 'trade'}));

// Subscribe to all crypto
ws.send(JSON.stringify({action: 'subscribe', symbols: ['BTC', 'ETH', 'SOL'], event: 'all'}));
```

### 3. Using Python

```python
import asyncio
import websockets
import json

async def test_websocket():
    uri = "ws://localhost:8080/ws"
    async with websockets.connect(uri) as ws:
        # Wait for connected message
        msg = await ws.recv()
        print(f"Connected: {msg}")

        # Subscribe to BTC trades
        await ws.send(json.dumps({
            "action": "subscribe",
            "symbols": ["BTC"],
            "event": "trade"
        }))

        # Listen for messages
        while True:
            msg = await ws.recv()
            data = json.loads(msg)
            print(f"Received: {data}")

asyncio.run(test_websocket())
```

### 4. Using curl (For Initial HTTP Check)

WebSocket upgrade check:
```bash
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  http://localhost:8080/ws
```

### 5. Load Testing with websocat

```bash
# Install websocat
cargo install websocat

# Connect and subscribe
echo '{"action":"subscribe","symbols":["BTC","ETH"],"event":"all"}' | \
  websocat ws://localhost:8080/ws

# Multiple connections for load testing
for i in {1..100}; do
  websocat ws://localhost:8080/ws &
done
```

---

## Performance Considerations

1. **Buffered Channels**: The broadcast channel is buffered (1000 messages) to handle bursts
2. **Message Dropping**: If a client's send buffer is full, messages are dropped (not queued indefinitely)
3. **Subscription Filtering**: Messages are only sent to clients that have subscribed to that symbol+event
4. **Wildcard Support**: Use `"*"` as symbol to receive all symbols (use carefully)

## Error Handling

- Connections automatically timeout after 60 seconds of inactivity
- Clients should send periodic ping messages to keep connection alive
- Server sends ping frames every 54 seconds
- Invalid JSON or unknown actions return error responses

## Security Notes

- In production, restrict `CheckOrigin` in upgrader to your domains only
- Consider adding JWT authentication via query parameter: `ws://localhost:8080/ws?token=<jwt>`
- Rate limit subscription requests to prevent abuse
