package market

import (
	"testing"
	"time"
)

func TestManagerDefaultSymbols(t *testing.T) {
	mgr := NewManager()
	mgr.DefaultSymbols()

	if mgr.SymbolCount() != 10 {
		t.Fatalf("expected 10 symbols, got %d", mgr.SymbolCount())
	}

	cryptos := mgr.ListByClass(Crypto)
	stocks := mgr.ListByClass(Stock)
	etfs := mgr.ListByClass(ETF)

	if len(cryptos) != 3 {
		t.Fatalf("expected 3 cryptos, got %d", len(cryptos))
	}
	if len(stocks) != 7 {
		t.Fatalf("expected 7 stocks, got %d", len(stocks))
	}
	if len(etfs) != 0 {
		t.Fatalf("expected 0 ETFs, got %d", len(etfs))
	}

	t.Logf("stocks: %v", stocks)

	// Clean up without starting
	mgr.StopAll()
	mgr.CloseAll()
}

func TestManagerAllSymbolsProduceOrders(t *testing.T) {
	mgr := NewManager()
	mgr.DefaultSymbols()
	mgr.StartAll()

	// Let all generators run for 2 seconds
	time.Sleep(2 * time.Second)
	mgr.StopAll()
	defer mgr.CloseAll()

	// Verify every symbol has orders in its book
	for _, symbol := range mgr.ListSymbols() {
		info := mgr.GetSymbol(symbol)
		info.Mu.Lock()
		depth := info.Book.GetDepth()
		info.Mu.Unlock()

		if len(depth.Bids) == 0 && len(depth.Asks) == 0 {
			t.Errorf("[%s] book is empty after 2s", symbol)
			continue
		}

		t.Logf("[%s] bids=%d asks=%d bestBid=%d bestAsk=%d",
			symbol, len(depth.Bids), len(depth.Asks), depth.BestBid, depth.BestAsk)
	}
}

func TestManagerAllSymbolsProduceTrades(t *testing.T) {
	mgr := NewManager()
	mgr.DefaultSymbols()
	mgr.StartAll()

	time.Sleep(3 * time.Second)
	mgr.StopAll()
	defer mgr.CloseAll()

	tradesTotal := 0
	for _, symbol := range mgr.ListSymbols() {
		info := mgr.GetSymbol(symbol)
		info.Mu.Lock()
		count := info.Book.LastExecutedCount()
		price := info.Book.LastExecutedPrice()
		info.Mu.Unlock()

		tradesTotal += count
		t.Logf("[%s] trades=%d lastPrice=%d", symbol, count, price)
	}

	if tradesTotal == 0 {
		t.Fatal("expected at least some trades across all symbols, got 0")
	}
	t.Logf("total trades across all symbols: %d", tradesTotal)
}

func TestManagerBooksAreIndependent(t *testing.T) {
	mgr := NewManager()

	// Add two symbols with very different prices
	mgr.AddSymbol("CHEAP", Stock, Config{
		InitialPrice: 10.0, Mu: 0.0, Sigma: 0.2,
		TickIntervalMS: 20, TickSize: 0.01,
		MinSpread: 0.01, MaxSpread: 0.05,
		MinQty: 1, MaxQty: 10, MaxOrdersPerSide: 3,
		AggressiveRate: 0.1,
	})
	mgr.AddSymbol("EXPENSIVE", Stock, Config{
		InitialPrice: 50000.0, Mu: 0.0, Sigma: 0.2,
		TickIntervalMS: 20, TickSize: 1.0,
		MinSpread: 10.0, MaxSpread: 50.0,
		MinQty: 1, MaxQty: 5, MaxOrdersPerSide: 3,
		AggressiveRate: 0.1,
	})

	mgr.StartAll()
	time.Sleep(1 * time.Second)
	mgr.StopAll()
	defer mgr.CloseAll()

	cheap := mgr.GetSymbol("CHEAP")
	expensive := mgr.GetSymbol("EXPENSIVE")

	cheap.Mu.Lock()
	cheapDepth := cheap.Book.GetDepth()
	cheap.Mu.Unlock()

	expensive.Mu.Lock()
	expDepth := expensive.Book.GetDepth()
	expensive.Mu.Unlock()

	// CHEAP should have prices around 1000 cents ($10)
	// EXPENSIVE should have prices around 5000000 cents ($50000)
	// They must NOT be mixed
	if cheapDepth.BestBid > 5000 {
		t.Errorf("CHEAP best bid %d is way too high, books may be mixed", cheapDepth.BestBid)
	}
	if expDepth.BestBid < 100000 {
		t.Errorf("EXPENSIVE best bid %d is way too low, books may be mixed", expDepth.BestBid)
	}

	t.Logf("CHEAP:     bestBid=%d bestAsk=%d", cheapDepth.BestBid, cheapDepth.BestAsk)
	t.Logf("EXPENSIVE: bestBid=%d bestAsk=%d", expDepth.BestBid, expDepth.BestAsk)
}
