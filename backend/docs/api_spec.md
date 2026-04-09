# Oak Capital API Documentation 📖
*Version: v1.0 | Base URL: `http://localhost:8080/api/v1`*

This document outlines the REST endpoints that interact with the C++ high-performance Limit Order Book.

### Order IDs
- **Create (market, limit add, stop add, stop-limit add):** the server assigns a monotonically increasing **`order_id` per symbol** and returns it in the response `data`. Do not send `order_id` in the request for these endpoints.
- **Cancel / modify:** pass the **`order_id`** returned when the order was created (per `symbol`).
- **Scope:** ids are unique among orders on the in-memory book for that symbol. If that book is destroyed and recreated (e.g. admin clears the symbol), numbering may restart at 1.

---

## 🛠 Global Data Structures

### Response Envelope
All API endpoints return JSON in a structured envelope to make frontend parsing predictable.
```json
// Success Response
{
  "success": true,
  "message": "limit order added",
  "data": { ... } // Varies by endpoint
}

// Error Response
{
  "success": false,
  "error": "invalid argument",
  "code": "ENGINE_ERR_INVALID_ARG"
}
```

### Trade
Executed trades are returned in order responses (`data.trades`). Each trade includes a UTC nanosecond timestamp assigned when the matching-engine call completes (trades from the same response share one timestamp).

```typescript
{
  "price": number,
  "qty": number,
  "maker_order_id": number,
  "taker_order_id": number,
  "timestamp_unix_nano": number  // Unix time in nanoseconds (UTC)
}
```

---

## 📈 Order Management Endpoints

### 1. Market Order
Executes immediately against the best available prices on the opposite side of the book.
- **Endpoint:** `POST /order/market`
- **Request Body:**
```json
{
  "symbol": "RELIANCE",
  "side": "buy",      // "buy" or "sell"
  "quantity": 5
}
```
- **Data Response:** (values shown are examples; `order_id` is assigned by the server)
```json
{
  "symbol": "RELIANCE",
  "order_id": 2,
  "side": "buy",
  "quantity": 5,
  "last_executed_count": 2,
  "trades": [
    { "price": 62000, "qty": 3, "maker_order_id": 1, "taker_order_id": 2, "timestamp_unix_nano": 1710000000000000000 },
    { "price": 62010, "qty": 2, "maker_order_id": 1, "taker_order_id": 2, "timestamp_unix_nano": 1710000000000000000 }
  ]
}
```

### 2. Add Limit Order
Places a resting limit order on the book. May execute immediately if marketable.
- **Endpoint:** `POST /order/limit/add`
- **Request Body:**
```json
{
  "symbol": "TCS",
  "side": "sell",
  "quantity": 10,
  "limit_price": 350000 // In integer ticks
}
```
- **Data Response:** includes server-assigned `order_id` (example: `"order_id": 1`) plus `trades` when the order crosses the book.

### 3. Cancel Limit Order
Removes a resting limit order from the book entirely.
- **Endpoint:** `POST /order/limit/cancel`
- **Request Body:**
```json
{
  "symbol": "TCS",
  "order_id": 1
}
```
(`order_id` must match the value returned from `POST /order/limit/add` for that symbol.)

### 4. Modify Limit Order
Adjusts the quantity or price of a resting limit order. *Note: Changing price typically loses time priority in standard LOB systems.*
- **Endpoint:** `POST /order/limit/modify`
- **Request Body:**
```json
{
  "symbol": "TCS",
  "order_id": 1,
  "quantity": 5,
  "limit_price": 349000
}
```

### 5. Stop Orders
Registers a stop market order that triggers when the asset's last traded price crosses the `stop_price`.
- **Add:** `POST /order/stop/add` -> `{symbol, side, quantity, stop_price}` — response `data` contains assigned `order_id`
- **Cancel:** `POST /order/stop/cancel` -> `{symbol, order_id}`
- **Modify:** `POST /order/stop/modify` -> `{symbol, order_id, quantity, stop_price}`

### 6. Stop-Limit Orders
Registers a stop limit order that rests until the `stop_price` is hit, and then converts into a standard limit order at `limit_price`.
- **Add:** `POST /order/stop-limit/add` -> `{symbol, side, quantity, limit_price, stop_price}` — response `data` contains assigned `order_id`
- **Cancel:** `POST /order/stop-limit/cancel` -> `{symbol, order_id}`
- **Modify:** `POST /order/stop-limit/modify` -> `{symbol, order_id, quantity, limit_price, stop_price}`

---

## 📊 Market Data Endpoints

### 7. Get Order Book Depth
Retrieves the full structural depth (up to 50 levels per side) for a specific symbol.
- **Endpoint:** `GET /book/depth?symbol=RELIANCE` *(also accepts POST)*
- **Data Response:**
```json
{
  "symbol": "RELIANCE",
  "Depth": {
    "best_bid": 6199000,
    "best_ask": 6200000,
    "last_price": 6199500,
    "bids": [
      { "price": 6199000, "volume": 12 },
      { "price": 6198500, "volume": 45 }
    ],
    "asks": [
      { "price": 6200000, "volume": 5 }
    ]
  }
}
```

### 8. Get Book Info
Retrieves a lightweight snapshot of the top of the book (L1 data).
- **Endpoint:** `GET /book/info?symbol=RELIANCE` *(also accepts POST)*
- **Data Response:**
```json
{
  "symbol": "RELIANCE",
  "best_bid": 6199000,
  "best_ask": 6200000,
  "last_executed_count": 1450,
  "last_executed_price": 6199500
}
```

### 9. List Active Books
Lists all trading pairs currently loaded into memory inside the Book Manager.
- **Endpoint:** `GET /book/list`
- **Data Response:**
```json
{
  "count": 3,
  "symbols": ["RELIANCE", "TCS", "HDFCBANK"]
}
```
