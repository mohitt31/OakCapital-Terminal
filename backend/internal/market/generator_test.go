package market

import (
	"sync"
	"testing"
	"time"

	"synthbull/internal/engine"
)

func setupBook(t *testing.T) *engine.Handle {
	t.Helper()
	book := engine.New()
	t.Cleanup(func() { book.Close() })
	return book
}

func TestGeneratorProducesOrders(t *testing.T) {
	book := setupBook(t)

	var mu sync.Mutex
	cfg := DefaultConfig()
	cfg.TickIntervalMS = 10

	gen := New(cfg, book, &mu)
	gen.SetStartOrderID(1000000)
	gen.Start()

	time.Sleep(200 * time.Millisecond)
	gen.Stop()

	mu.Lock()
	depth := book.GetDepth()
	mu.Unlock()

	if len(depth.Bids) == 0 && len(depth.Asks) == 0 {
		t.Fatal("expected orders in the book")
	}
}

func TestGeneratorProducesTrades(t *testing.T) {
	book := setupBook(t)

	var mu sync.Mutex
	cfg := DefaultConfig()
	cfg.TickIntervalMS = 10
	cfg.AggressiveRate = 1.0 // force trades every tick

	gen := New(cfg, book, &mu)
	gen.SetStartOrderID(2000000)
	gen.Start()

	time.Sleep(1000 * time.Millisecond)
	gen.Stop()

	mu.Lock()
	execCount := book.LastExecutedCount()
	mu.Unlock()

	if execCount == 0 {
		t.Fatal("expected trades")
	}
}

func TestGeneratorStopsCleanly(t *testing.T) {
	book := setupBook(t)

	var mu sync.Mutex
	gen := New(DefaultConfig(), book, &mu)
	gen.Start()

	time.Sleep(50 * time.Millisecond)
	gen.Stop()

	mu.Lock()
	countAfterStop := book.LastExecutedCount()
	mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	countLater := book.LastExecutedCount()
	mu.Unlock()

	if countLater != countAfterStop {
		t.Fatalf("generator still running after Stop()")
	}
}
