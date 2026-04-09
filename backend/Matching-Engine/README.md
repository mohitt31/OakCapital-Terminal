# C++ Matching Engine

A high-performance Limit Order Book implemented in C++, capable of handling over **1,400,000 orders/sec**. Supports Market, Limit, Stop, and Stop-Limit orders with Price-Time (FIFO) priority matching.

Integrated into the Go backend via CGO — see `internal/engine/cgo_bridge.go` and the C API in `include/engine_c_api.h`.

## Order Types

- **Market Order** — executes immediately at the best available price.
- **Limit Order** (Add, Modify, Cancel) — rests in the book at a specified price; matches only if crossed.
- **Market-Limit Order** — a limit order that crosses the spread and executes as a market order.
- **Stop Order** (Add, Modify, Cancel) — converts to a Market order when the market price crosses the stop price.
- **Stop-Limit Order** (Add, Modify, Cancel) — converts to a Limit order when the market price crosses the stop price.

## Architecture

```
Matching-Engine/
├── Limit_Order_Book/       ← core engine (Book, Limit, Order)
├── internal/               ← internal headers used by C API
├── src/
│   ├── Book.cpp / Limit.cpp / Order.cpp
│   ├── engine_c_api.cpp    ← C ABI wrapper for CGO
│   └── HTTPServer.cpp      ← standalone HTTP server (debug only)
├── include/
│   ├── engine_c_api.h      ← exported C API
│   └── engine_types.h      ← shared types (TradeResult, etc.)
├── tests/
│   ├── smoke_main.cpp      ← smoke test: add/cross/verify
│   └── bookTests.cpp
├── Generate_Orders/        ← test data generator (not used in production)
├── Process_Orders/         ← file-based order pipeline (not used in production)
├── CMakeLists.txt
└── main.cpp                ← standalone entry point
```

## Data Structures

```cpp
Order
    int idNumber;
    bool buyOrSell;
    int shares;
    int limit;
    Order *nextOrder, *prevOrder;
    Limit *parentLimit;

Limit                        // one price level — AVL tree node + order queue
    int limitPrice;
    int size, totalVolume;
    Limit *parent, *leftChild, *rightChild;
    Order *headOrder, *tailOrder;

Book
    Limit *buyTree, *sellTree;
    Limit *lowestSell, *highestBuy;
    Limit *stopBuyTree, *stopSellTree;
    Limit *lowestStopBuy, *highestStopSell;
```

A binary (AVL) tree of `Limit` objects sorted by price, each holding a doubly-linked list of `Order` objects. Separate trees for buy and sell sides. The best bid/ask are tracked via `highestBuy` / `lowestSell` pointers for O(1) retrieval.

### Complexity

| Operation          | Complexity |
|--------------------|-----------|
| Add Order (new price level) | O(log M) |
| Add Order (existing level)  | O(1) |
| Cancel Order       | O(1) |
| Modify Order       | O(1) |
| Execute            | O(1) |
| GetVolumeAtLimit   | O(1) |
| GetBestBid/Offer   | O(1) |

M = number of distinct price levels (much smaller than total order count).

## Building

```bash
cd Matching-Engine
mkdir -p build && cd build
cmake ..
make
```

The build produces:
- `libmatching_engine_core.a` — static library (linked by CGO)
- `libmatching_engine_c_api.dylib` — shared library
- `matching_engine_smoke` — smoke test binary

## C API (CGO Integration)

The Go backend calls the engine via the C ABI defined in `include/engine_c_api.h`. See `src/engine_c_api.cpp` for the implementation and `LOB_API.md` for the full HTTP endpoint reference.

## Assumptions

- Order shares must be greater than 0.
- Limit and stop prices must be greater than 0.
- Order ID numbers must be unique per book instance.
