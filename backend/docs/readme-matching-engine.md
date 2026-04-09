# C++ Matching Engine — Complete Technical Reference

This document explains every class, data structure, and algorithm in the `Limit-Order-Book/` codebase so any team member can understand and modify it.

---

## Architecture Overview

```
main.cpp               <- Entry point (thin, just wiring)
   |
   ├── GenerateOrders   <- Creates test data (text files) — NOT needed for our project
   |
   └── OrderPipeline    <- Reads text files, parses lines, routes to Book — NOT needed for our project
          |
          └── Book      <- THE matching engine (this is what we use via CGO)
               |
               ├── Limit   <- One price level (AVL tree node + order queue)
               |    |
               |    └── Order  <- Single order (doubly-linked list node)
               |
               └── [AVL tree balancing, edge tracking, stop order logic]
```

**For our Go integration, we only care about `Book`, `Limit`, and `Order`.** The `OrderPipeline` and `GenerateOrders` are file-based utilities we replace with our Go services.

---

## File Map

```
Limit-Order-Book/
├── Limit_Order_Book/
│   ├── Order.hpp / Order.cpp     <- Single order (linked list node)
│   ├── Limit.hpp / Limit.cpp     <- Price level (AVL tree node + order queue)
│   └── Book.hpp  / Book.cpp      <- The matching engine (all the logic)
│
├── Process_Orders/
│   └── OrderPipeline.hpp / .cpp  <- Text file parser → routes to Book methods
│
├── Generate_Orders/
│   ├── GenerateOrders.hpp / .cpp <- Random order generator for benchmarking
│   └── initialOrders.txt         <- Sample initial orders
│
├── test/                         <- GoogleTest test suites
├── CMakeLists.txt                <- Build configuration (C++20, -O2)
└── main.cpp                      <- Entry point
```

---

## Class 1: Order — The Smallest Unit

**File:** `Limit_Order_Book/Order.hpp` / `Order.cpp`

Think of an Order as a **single sticky note** on a trading board. It knows what it wants and where it sits in the queue.

### Fields

| Field | Type | Purpose |
|-------|------|---------|
| `idNumber` | `int` | Unique order ID (e.g., 42) |
| `buyOrSell` | `bool` | `true` = buy, `false` = sell |
| `shares` | `int` | Quantity (e.g., 100 shares) |
| `limit` | `int` | Price (e.g., 302). For stop orders, this is 0 |
| `nextOrder` | `Order*` | Next order at the SAME price (FIFO chain →) |
| `prevOrder` | `Order*` | Previous order at the SAME price (← FIFO chain) |
| `parentLimit` | `Limit*` | Which price level this order belongs to |

### How Orders Form a Chain (Doubly-Linked List)

Orders at the same price are connected in a doubly-linked list. This is how **time priority** works — the oldest order is at the head, newest at the tail:

```
Price $302:  Order#5 ←→ Order#8 ←→ Order#12 ←→ Order#15
             (head)                              (tail)
             ↑ matched first                     ↑ matched last
             (oldest)                            (newest)
```

### Methods

**`execute()`** — Called when this order is fully filled by a market order.
- Removes itself from the HEAD of the linked list
- Updates parentLimit's headOrder to point to nextOrder
- Decrements parentLimit's totalVolume and size
- After this, the Order object gets deleted by Book

**`cancel()`** — Called when user cancels this order.
- Can remove from ANYWHERE in the linked list (not just head)
- Repoints prev→next and next→prev around itself
- Decrements parentLimit's totalVolume and size
- This is O(1) because we have direct prev/next pointers

**`partiallyFillOrder(shares)`** — Called when only part of the order is matched.
- Reduces `this->shares` by the matched amount
- Calls `parentLimit->partiallyFillTotalVolume()` to update the price level's total
- Order stays in the book with reduced quantity

**`modifyOrder(newShares, newLimit)`** — Resets the order for re-insertion at a new price.
- Updates shares and limit price
- Clears all pointers (next, prev, parentLimit) — the order will be re-appended to a new Limit

---

## Class 2: Limit — One Price Level

**File:** `Limit_Order_Book/Limit.hpp` / `Limit.cpp`

A Limit serves **double duty** — it's both:
1. A **price level** holding a queue of orders
2. A **node in an AVL tree** for fast price lookup

### Fields

| Field | Type | Purpose |
|-------|------|---------|
| `limitPrice` | `int` | The price this level represents (e.g., 302) |
| `size` | `int` | Number of orders at this price |
| `totalVolume` | `int` | Sum of all shares across all orders at this price |
| `buyOrSell` | `bool` | Whether this is a buy or sell level |
| **AVL tree pointers:** | | |
| `parent` | `Limit*` | Parent node in the AVL tree |
| `leftChild` | `Limit*` | Left child (lower prices) |
| `rightChild` | `Limit*` | Right child (higher prices) |
| **Order queue pointers:** | | |
| `headOrder` | `Order*` | First (oldest) order — matched first (FIFO) |
| `tailOrder` | `Order*` | Last (newest) order — matched last |

### Visual: Limit as Both Tree Node and Order Queue

```
         AVL Tree                    Order Queue inside each node
         ────────                    ─────────────────────────────
            $302                     $302: [Order#5] → [Order#8] → [Order#12]
           /    \                          (head)                   (tail)
        $295    $310
        /         \                  $295: [Order#1] → [Order#9]
     $290         $315
                                     $310: [Order#2] → [Order#7] → [Order#14] → [Order#16]
```

### Methods

**`append(order)`** — Adds a new order to the TAIL of the queue.
```
Before: head → [#5] → [#8] → [#12] ← tail
After:  head → [#5] → [#8] → [#12] → [#15] ← tail
                                       ↑ new order appended
```
- Sets `order->parentLimit = this`
- Increments `size` and `totalVolume`
- This preserves **time priority** — newer orders go to the back

**`~Limit()` (destructor)** — Handles BST node deletion when a price level is emptied.
This is the most complex method. When all orders at a price are filled/cancelled, the Limit node must be removed from the AVL tree. Three cases:
1. **No children** → just remove
2. **One child** → replace with child
3. **Two children** → find in-order successor (smallest node in right subtree), swap it in

---

## Class 3: Book — The Matching Engine

**File:** `Limit_Order_Book/Book.hpp` / `Book.cpp`

This is the brain. It manages 4 AVL trees and 4 hash maps.

### Internal Data Structures

```
Book
│
│  ═══ LIMIT ORDERS (regular buy/sell) ═══
├── buyTree            → AVL tree root for all buy price levels
├── sellTree           → AVL tree root for all sell price levels
├── highestBuy         → direct pointer to best bid — O(1) access
├── lowestSell         → direct pointer to best ask — O(1) access
├── limitBuyMap        → HashMap<price → Limit*> — find buy level by price O(1)
├── limitSellMap       → HashMap<price → Limit*> — find sell level by price O(1)
│
│  ═══ STOP ORDERS ═══
├── stopBuyTree        → AVL tree for buy stop levels
├── stopSellTree       → AVL tree for sell stop levels
├── lowestStopBuy      → edge pointer for stop buys
├── highestStopSell    → edge pointer for stop sells
├── stopMap            → HashMap<price → Limit*>
│
│  ═══ UNIVERSAL ORDER LOOKUP ═══
└── orderMap           → HashMap<orderId → Order*> — find ANY order by ID in O(1)
```

### Visual: The Full Order Book

```
                SELL TREE (AVL)
                    $310
                   /    \
                $305    $315
                /         \
             $302         $320 ← worst ask
               ↑
          lowestSell (best ask) = $302

          ─────── SPREAD ($302 - $298 = $4) ───────

          highestBuy (best bid) = $298
               ↓
             $298         $280 ← worst bid
                \         /
                $295    $285
                   \    /
                    $290
                BUY TREE (AVL)
```

### Performance (Why This Design)

| Operation | Complexity | How |
|-----------|------------|-----|
| Get best bid/ask | **O(1)** | Direct pointer (`highestBuy`, `lowestSell`) |
| Find order by ID | **O(1)** | `orderMap` hash map lookup |
| Find price level | **O(1)** | `limitBuyMap`/`limitSellMap` hash map lookup |
| Cancel order | **O(1)** | Find in orderMap → call `order->cancel()` (linked list removal) |
| Add order to existing price | **O(1)** | Find Limit in map → `limit->append(order)` |
| Add order at new price | **O(log n)** | Insert new Limit into AVL tree |
| Execute match | **O(1) per fill** | Walk head orders at best price |
| AVL rebalance | **O(log n)** | After insert/delete, walk up and rotate |

The system achieves **1.4 million orders/sec** with this design.

---

### Public API — The Methods We Call From Go

#### Core Order Methods (required by PS)

**`addLimitOrder(orderId, buyOrSell, shares, limitPrice)`**
Places a limit order. If it crosses the spread (e.g., buy at $305 when best ask is $302), it first acts as a market order eating through available asks, then any remaining shares rest in the book.

Flow:
```
addLimitOrder(42, BUY, 100, $305)
│
├── 1. Can it cross? Is $305 >= lowestSell ($302)?
│   └── YES → limitOrderAsMarketOrder()
│       → Eats asks at $302, $305 until shares exhausted or no more asks ≤ $305
│       → Returns remaining shares
│
├── 2. remaining shares > 0?
│   └── Create Order(42, BUY, remaining, $305)
│   └── Add to orderMap
│
├── 3. Does price level $305 exist in limitBuyMap?
│   ├── NO  → Create new Limit($305), insert into buyTree AVL, update highestBuy if needed
│   └── YES → just use existing Limit
│
└── 4. limit->append(order) — order goes to tail of queue at $305
```

**`marketOrder(orderId, buyOrSell, shares)`**
Immediately matches against the best available orders on the opposite side.

Flow:
```
marketOrder(99, BUY, 500)
│
├── 1. Look at lowestSell (best ask, e.g., $302 with 200 shares)
│
├── 2. LOOP while shares > 0 and lowestSell exists:
│   ├── Get headOrder from lowestSell (oldest order at best ask)
│   │
│   ├── headOrder.shares (80) ≤ remaining (500)?
│   │   └── YES → execute headOrder (remove it), remaining = 420
│   │       └── If that Limit is now empty → deleteLimit (remove from AVL tree)
│   │
│   └── headOrder.shares (300) > remaining (120)?
│       └── Partially fill: headOrder.shares -= 120, done
│
├── 3. After matching, check if any stop orders should trigger
│   └── executeStopOrders()
│
└── 4. If book is empty and shares remain → unfilled portion is lost (no resting)
```

**`cancelLimitOrder(orderId)`**
Removes an order from the book by ID.

Flow:
```
cancelLimitOrder(42)
│
├── 1. Find order in orderMap → O(1)
├── 2. order->cancel() → removes from linked list at its price level → O(1)
├── 3. If that Limit is now empty (size == 0) → deleteLimit from AVL tree
├── 4. Remove from orderMap
└── 5. delete order (free memory)
```

#### State Reading Methods (needed for frontend)

```cpp
book->getHighestBuy()                    → Limit* (best bid price level)
book->getLowestSell()                    → Limit* (best ask price level)
book->getHighestBuy()->getLimitPrice()   → int (best bid price, e.g., 298)
book->getLowestSell()->getLimitPrice()   → int (best ask price, e.g., 302)
book->getHighestBuy()->getTotalVolume()  → int (total shares at best bid)
book->getLowestSell()->getTotalVolume()  → int (total shares at best ask)
book->searchOrderMap(id)                 → Order* (find any order)
book->inOrderTreeTraversal(buyTree)      → vector<int> (all bid prices, sorted ascending)
book->inOrderTreeTraversal(sellTree)     → vector<int> (all ask prices, sorted ascending)
```

#### Extra Methods (not required by PS but available)

```cpp
book->modifyLimitOrder(orderId, newShares, newLimit)
book->addStopOrder(orderId, buyOrSell, shares, stopPrice)
book->cancelStopOrder(orderId)
book->modifyStopOrder(orderId, newShares, newStopPrice)
book->addStopLimitOrder(orderId, buyOrSell, shares, limitPrice, stopPrice)
book->cancelStopLimitOrder(orderId)
book->modifyStopLimitOrder(orderId, newShares, newLimitPrice, newStopPrice)
```

---

### The Matching Algorithm: `marketOrderHelper`

This is the actual function that executes trades. Located at `Book.cpp:1002-1025`.

```cpp
void Book::marketOrderHelper(int orderId, bool buyOrSell, int shares)
{
    // bookEdge = lowestSell (for buys) or highestBuy (for sells)
    auto& bookEdge = buyOrSell ? lowestSell : highestBuy;

    // Fully fill orders until we run out of shares
    while (bookEdge != nullptr && bookEdge->getHeadOrder()->getShares() <= shares)
    {
        Order* headOrder = bookEdge->getHeadOrder();
        shares -= headOrder->getShares();
        headOrder->execute();                    // Remove from linked list
        if (bookEdge->getSize() == 0)
            deleteLimit(bookEdge);               // Remove empty price level from AVL tree
        deleteFromOrderMap(headOrder->getOrderId());
        delete headOrder;                        // Free memory
        executedOrdersCount += 1;
    }

    // Partially fill the last order if shares remain
    if (bookEdge != nullptr && shares != 0)
    {
        bookEdge->getHeadOrder()->partiallyFillOrder(shares);
        executedOrdersCount += 1;
    }
}
```

**Step-by-step example:**

```
Market BUY 500 shares

SELL BOOK:
  $302: [Order#5: 80sh] → [Order#8: 120sh]    ← lowestSell
  $305: [Order#2: 200sh]
  $310: [Order#7: 500sh]

Pass 1: headOrder = Order#5 (80sh at $302)
  80 ≤ 500 → execute Order#5, remaining = 420

Pass 2: headOrder = Order#8 (120sh at $302)
  120 ≤ 420 → execute Order#8, remaining = 300
  $302 level now empty → deleteLimit($302)
  lowestSell now points to $305

Pass 3: headOrder = Order#2 (200sh at $305)
  200 ≤ 300 → execute Order#2, remaining = 100
  $305 level now empty → deleteLimit($305)
  lowestSell now points to $310

Pass 4: headOrder = Order#7 (500sh at $310)
  500 > 100 → partially fill: Order#7 now has 400sh, done

Result: 4 trades executed
  80 @ $302, 120 @ $302, 200 @ $305, 100 @ $310
```

---

### AVL Tree Balancing

The AVL tree keeps the price level tree balanced so operations stay O(log n). Four rotation types:

```
LL Rotation (left-left case):
      C               B
     /               / \
    B       →       A   C
   /
  A

RR Rotation (right-right case):
  A                 B
   \               / \
    B      →      A   C
     \
      C

LR Rotation (left-right case):
      C               B
     /               / \
    A       →       A   C
     \
      B

RL Rotation (right-left case):
  A                 B
   \               / \
    C      →      A   C
   /
  B
```

The `balance()` function checks if the height difference between left and right subtrees exceeds 1, and applies the appropriate rotation. This runs after every insert and delete, walking up the tree from the affected node.

---

### Stop Orders — How They Work

Stop orders sit dormant until the market price crosses their trigger price, then they activate:

- **Stop Market Order**: When triggered, becomes a market order
- **Stop Limit Order**: When triggered, becomes a limit order

```
Example: Stop Buy at $310

Market is at $302 ask. Stop order waits.
A market buy pushes the ask up to $311.
Now $310 ≤ $311 (lowestSell), so the stop triggers:
  → Stop market order → immediate market buy
  → Stop limit order → placed as limit buy at the stored limitPrice
```

`executeStopOrders()` is called after every market order and checks if any stops should fire.

---

## Class 4: OrderPipeline — Text File Parser

**File:** `Process_Orders/OrderPipeline.hpp` / `.cpp`

**We don't need this for our project.** It just parses text files line by line:

```
"AddLimit 42 1 100 302"  →  book->addLimitOrder(42, true, 100, 302)
"Market 99 1 500"        →  book->marketOrder(99, true, 500)
"CancelLimit 42"         →  book->cancelLimitOrder(42)
```

Uses a hash map of function pointers for fast string→function dispatch. Also times each operation and writes to `order_processing_times.csv` for benchmarking.

**Replaced by:** Our CGO bridge calls Book methods directly from Go.

---

## Class 5: GenerateOrders — Test Data Generator

**File:** `Generate_Orders/GenerateOrders.hpp` / `.cpp`

**We don't need this for our project.** It generates random orders for benchmarking:

- Prices: normal distribution centered at $300, std dev 50
- Shares: uniform random 1-1000
- Order type probabilities:
  - 29.5% AddLimit (most common — builds the book)
  - 19.5% CancelLimit
  - 12% each for stop operations
  - 2.5% Market orders (rarest — consumes liquidity)
  - 2.5% MarketLimit (limit orders that cross the spread)

**Replaced by:** Our GBM market generator in Go (`internal/market/generator.go`).

---

## What We Need to Modify for Our Project

### 1. Trade Event Emission (CRITICAL)

Currently, `marketOrderHelper` executes trades but **doesn't report what happened**. It just deletes orders and increments a counter. We need it to **collect trade data** so Go can broadcast to the frontend.

What we need to capture per trade:
```
{
    price:      302,        // execution price
    quantity:   80,         // shares traded
    buyOrderId: 99,         // aggressive order (taker)
    sellOrderId: 5,         // passive order (maker)
    timestamp:  <now>       // when it happened
}
```

We'll modify `marketOrderHelper` (or the C wrapper) to fill a trade buffer that Go reads after each call.

### 2. Order Book Snapshot

We need a way to get the full order book depth for the frontend visualization:
```
Bids: [{price: 298, volume: 850}, {price: 295, volume: 375}, ...]
Asks: [{price: 302, volume: 200}, {price: 305, volume: 500}, ...]
```

We'll add a C wrapper function that walks both trees and fills a buffer with price+volume pairs.

### 3. Decimal Price Support

Currently prices are **integers only** (e.g., 302 not 302.50). For a crypto exchange we might want fractional prices. Options:
- Use integers as "cents" (multiply by 100, so $302.50 = 30250)
- Modify the C++ to use doubles (more changes, possible floating point issues)
- Keep integers — simpler, works fine for the competition

**Recommendation:** Keep integers, treat them as cents. $302.50 → store as 30250, divide by 100 in the frontend.

---

## Input Format Reference

If you ever need to test the engine standalone with text files:

```
Market <orderId> <buyOrSell> <shares>
AddLimit <orderId> <buyOrSell> <shares> <limitPrice>
AddMarketLimit <orderId> <buyOrSell> <shares> <limitPrice>
CancelLimit <orderId>
ModifyLimit <orderId> <newShares> <newLimit>
AddStop <orderId> <buyOrSell> <shares> <stopPrice>
CancelStop <orderId>
ModifyStop <orderId> <newShares> <newStopPrice>
AddStopLimit <orderId> <buyOrSell> <shares> <limitPrice> <stopPrice>
CancelStopLimit <orderId>
ModifyStopLimit <orderId> <newShares> <newLimitPrice> <newStopPrice>
```

Where `buyOrSell`: `1` = buy, `0` = sell. All values are integers.

Example `initialOrders.txt`:
```
AddLimit 1 1 976 278    (Order #1, Buy, 976 shares at $278)
AddLimit 2 1 94 298     (Order #2, Buy, 94 shares at $298)
AddLimit 3 0 187 340    (Order #3, Sell, 187 shares at $340)
```
