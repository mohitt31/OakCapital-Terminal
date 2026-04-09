# Limit Order Book — HTTP Server API

C++ matching engine exposing a REST API via `cpp-httplib`. Supports multiple independent order books (one per symbol), limit orders, market orders, and cancellations.

---

## Setup & Running

### Prerequisites
- GCC / MinGW with C++17 support
- CMake 3.10+
- [`cpp-httplib`](https://github.com/yhirose/cpp-httplib) (header-only)
- [`nlohmann/json`](https://github.com/nlohmann/json) (header-only)

### Build & Run

Server starts on:
```
http://0.0.0.0:8080
```

---

## Base URL

```
http://localhost:8080
```

---

## Endpoints

### `GET /health`

Check that the server is running.

**Request** — no body needed.

**Response `200`**
```json
{ "status": "ok" }
```

---

### `POST /book/:symbol`

Create a new order book for a trading symbol. Must be called before placing any orders for that symbol.

**URL parameter**

| Parameter | Type   | Description                        |
|-----------|--------|------------------------------------|
| `symbol`  | string | Ticker symbol e.g. `AAPL`, `BTC`  |

**Request** — no body needed.

**Response `200`** — book created
```json
{ "status": "created", "symbol": "AAPL" }
```

**Response `400`** — book already exists
```json
{ "error": "Order book already exists for symbol" }
```

---

### `GET /books`

List all symbols that have an active order book.

**Request** — no body needed.

**Response `200`**
```json
{ "books": ["AAPL", "BTC", "ETH"] }
```

---

### `GET /book/:symbol`

Get the current state of an order book — best bid/ask and full depth on both sides.

**URL parameter**

| Parameter | Type   | Description    |
|-----------|--------|----------------|
| `symbol`  | string | Ticker symbol  |

**Request** — no body needed.

**Response `200`**
```json
{
  "symbol": "AAPL",
  "timestamp": 1742134800,
  "best_bid": 49900,
  "best_ask": 50100,
  "bids": [
    { "price": 49900, "volume": 40 },
    { "price": 49800, "volume": 50 }
  ],
  "asks": [
    { "price": 50100, "volume": 80 },
    { "price": 50200, "volume": 120 }
  ]
}
```

> `best_bid` / `best_ask` return `-1` when that side of the book is empty.  
> `bids` are sorted highest price first. `asks` are sorted lowest price first.

**Response `404`** — symbol not found
```json
{ "error": "Order book not found" }
```

---

### `POST /order/limit`

Place a limit order. If the order crosses the book (buy price ≥ lowest ask, or sell price ≤ highest bid), it executes immediately as a market order. Any unfilled remainder rests in the book.

**Headers**
```
Content-Type: application/json
```

**Request body**

| Field        | Type    | Required | Description                          |
|--------------|---------|----------|--------------------------------------|
| `symbol`     | string  | yes      | Ticker symbol                        |
| `orderId`    | integer | yes      | Unique order ID (caller-assigned)    |
| `buyOrSell`  | boolean | yes      | `true` = buy, `false` = sell         |
| `shares`     | integer | yes      | Number of shares (must be > 0)       |
| `limitPrice` | integer | yes      | Limit price in cents (must be > 0)   |

**Example — buy limit (will rest if no matching ask)**
```json
{
  "symbol": "AAPL",
  "orderId": 1,
  "buyOrSell": true,
  "shares": 100,
  "limitPrice": 49900
}
```

**Example — sell limit (will cross and match immediately)**
```json
{
  "symbol": "AAPL",
  "orderId": 4,
  "buyOrSell": false,
  "shares": 60,
  "limitPrice": 49900
}
```

**Response `200`**
```json
{ "status": "order_added", "orderId": 1, "symbol": "AAPL" }
```

**Response `400`** — missing or invalid fields
```json
{ "error": "Invalid or missing fields" }
```

**Response `400`** — zero/negative values
```json
{ "error": "Invalid input parameters" }
```

**Response `500`** — unexpected internal error
```json
{ "error": "<exception message>" }
```

---

### `POST /order/market`

Place a market order. Executes immediately at the best available price, walking through price levels until filled or the book is exhausted. Any unfilled quantity is silently dropped.

**Headers**
```
Content-Type: application/json
```

**Request body**

| Field       | Type    | Required | Description                        |
|-------------|---------|----------|------------------------------------|
| `symbol`    | string  | yes      | Ticker symbol                      |
| `orderId`   | integer | yes      | Unique order ID (caller-assigned)  |
| `buyOrSell` | boolean | yes      | `true` = buy, `false` = sell       |
| `shares`    | integer | yes      | Number of shares (must be > 0)     |

**Example — market buy**
```json
{
  "symbol": "AAPL",
  "orderId": 5,
  "buyOrSell": true,
  "shares": 30
}
```

**Response `200`**
```json
{ "status": "market_order_executed", "orderId": 5, "symbol": "AAPL" }
```

**Response `400`** — missing or invalid fields
```json
{ "error": "Invalid or missing fields" }
```

---

### `DELETE /order/:symbol/:orderId`

Cancel a resting limit order. The order is removed from the book immediately.

**URL parameters**

| Parameter | Type    | Description                     |
|-----------|---------|---------------------------------|
| `symbol`  | string  | Ticker symbol                   |
| `orderId` | integer | ID of the order to cancel       |

**Request** — no body needed.

**Response `200`**
```json
{ "status": "order_cancelled", "orderId": 2 }
```

**Response `404`** — symbol not found
```json
{ "error": "Order book not found for symbol" }
```

> If `orderId` doesn't exist in the book, the server still returns `200` — no error is raised. The Book internally logs "No order number X" to stdout.

---

## Typical Request Sequence

The intended order of operations for a new symbol:

```
1.  POST   /book/AAPL              → create the book
2.  POST   /order/limit            → place buy @ 49900 (rests)
3.  POST   /order/limit            → place sell @ 50100 (rests)
4.  GET    /book/AAPL              → view spread: bid=49900, ask=50100
5.  POST   /order/limit            → place sell @ 49900 (crosses → matches order from step 2)
6.  GET    /book/AAPL              → verify fill: volume at 49900 reduced
7.  POST   /order/market           → market buy eats into asks
8.  DELETE /order/AAPL/2           → cancel a resting order
9.  GET    /book/AAPL              → confirm final book state
```

---

## Key Behaviors

**Price-Time (FIFO) priority** — within a price level, the order placed earliest fills first.

**Crossing limit orders** — a limit order that crosses the spread is treated internally as a market order via `limitOrderAsMarketOrder()`. It will never rest at a price that would create a crossed book.

**Partial fills** — if a market order or crossing limit order consumes only part of a resting order, the remainder stays in the book at the original price.

**Empty book market orders** — if a market order cannot be filled (no liquidity on the opposite side), the unfilled shares are silently dropped. No error is returned.

**Auto-created books** — the `/order/limit` and `/order/market` endpoints will silently create a book for a symbol if one doesn't exist yet. Using `POST /book/:symbol` first is recommended but not required.

**Stop orders** — the underlying `Book` class supports stop and stop-limit orders (`addStopOrder`, `addStopLimitOrder`) but these are not yet exposed via HTTP endpoints.

---

## Error Reference

| HTTP Status | Meaning                                      |
|-------------|----------------------------------------------|
| `200`       | Success                                      |
| `400`       | Bad request — missing fields, invalid values, or duplicate book |
| `404`       | Symbol not found                             |
| `500`       | Internal C++ exception — check server logs   |

---

## Notes for Go Backend Integration

- Prices are **integers in cents** (e.g. `50000` = $500.00). Confirm this convention with your team.
- `orderId` must be **globally unique** across all symbols — the underlying `orderMap` is per-book, so reusing IDs across symbols is safe, but reusing within the same symbol will silently overwrite.
- The server uses a **single global mutex** (`booksMutex`) — all endpoints are serialized. This is safe for the competition's 50–100 msg/sec target but would need per-book locking for higher throughput.
- Trade events are **not yet emitted** in HTTP responses — fills happen internally but the response only confirms the order was processed. Go will need to poll `GET /book/:symbol` or a future WebSocket endpoint to observe fills.