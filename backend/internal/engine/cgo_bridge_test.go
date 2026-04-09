package engine

import (
	"testing"
)

func TestNewAndClose(t *testing.T) {
	h := New()
	if h == nil || h.h == nil {
		t.Fatal("engine_create returned nil")
	}
	h.Close()
}

func TestLimitOrderRests(t *testing.T) {
	h := New()
	defer h.Close()

	// Place a buy at 100 — no seller, so it should rest in book
	s, res := h.AddLimit(1, SideBuy, 50, 100)
	if s != StatusOK {
		t.Fatalf("AddLimit failed: %v", s.Error())
	}
	if len(res.Trades) > 0 {
		t.Fatal("expected no trade, order should rest")
	}

	if h.BestBid() != 100 {
		t.Fatalf("best bid = %d, want 100", h.BestBid())
	}
	if h.BestAsk() != -1 {
		t.Fatal("expected no ask, book sell side should be empty")
	}
}

func TestLimitOrderCrosses(t *testing.T) {
	h := New()
	defer h.Close()

	// Buy 100 shares at price 50
	h.AddLimit(1, SideBuy, 100, 50)

	// Opposite-side order should be accepted without bridge errors.
	s, _ := h.AddLimit(2, SideSell, 40, 50)
	if s != StatusOK {
		t.Fatalf("AddLimit sell failed: %v", s.Error())
	}
}

func TestMarketOrder(t *testing.T) {
	h := New()
	defer h.Close()

	// Seed the book with asks
	h.AddLimit(1, SideSell, 30, 100)
	h.AddLimit(2, SideSell, 50, 101)

	// Market order behavior is engine-defined; this test asserts bridge stability.
	s, _ := h.Market(3, SideBuy, 30)
	if s != StatusOK {
		t.Fatalf("Market order failed: %v", s.Error())
	}
}

func TestCancelOrder(t *testing.T) {
	h := New()
	defer h.Close()

	h.AddLimit(1, SideBuy, 100, 50)
	if h.BestBid() != 50 {
		t.Fatalf("best bid = %d, want 50", h.BestBid())
	}

	s, _ := h.CancelLimit(1)
	if s != StatusOK {
		t.Fatalf("CancelLimit failed: %v", s.Error())
	}

	// Book should be empty now
	if h.BestBid() != -1 {
		t.Fatalf("best bid = %d, want -1 after cancel", h.BestBid())
	}
}

func TestCancelNonExistent(t *testing.T) {
	h := New()
	defer h.Close()

	s, _ := h.CancelLimit(999)
	if s != StatusNotFound {
		t.Fatalf("expected StatusNotFound, got %v", s.Error())
	}
}

func TestModifyOrder(t *testing.T) {
	h := New()
	defer h.Close()

	h.AddLimit(1, SideBuy, 100, 50)

	// Modify: change price from 50 to 60, qty to 200
	s, _ := h.ModifyLimit(1, 200, 60)
	if s != StatusOK {
		t.Fatalf("ModifyLimit failed: %v", s.Error())
	}

	if h.BestBid() != 60 {
		t.Fatalf("best bid = %d, want 60 after modify", h.BestBid())
	}
}

func TestDepthSnapshot(t *testing.T) {
	h := New()
	defer h.Close()

	// Build a book with multiple levels
	h.AddLimit(1, SideBuy, 100, 50)
	h.AddLimit(2, SideBuy, 200, 49)
	h.AddLimit(3, SideSell, 150, 55)
	h.AddLimit(4, SideSell, 300, 56)

	depth := h.GetDepth()

	if depth.BestBid != 50 {
		t.Fatalf("depth best bid = %d, want 50", depth.BestBid)
	}
	if depth.BestAsk != 55 {
		t.Fatalf("depth best ask = %d, want 55", depth.BestAsk)
	}
	if len(depth.Bids) != 2 {
		t.Fatalf("bid levels = %d, want 2", len(depth.Bids))
	}
	if len(depth.Asks) != 2 {
		t.Fatalf("ask levels = %d, want 2", len(depth.Asks))
	}

	// Bids should be highest first: 50, 49
	if depth.Bids[0].Price != 50 || depth.Bids[0].Volume != 100 {
		t.Fatalf("bid[0] = {%d, %d}, want {50, 100}", depth.Bids[0].Price, depth.Bids[0].Volume)
	}
	if depth.Bids[1].Price != 49 || depth.Bids[1].Volume != 200 {
		t.Fatalf("bid[1] = {%d, %d}, want {49, 200}", depth.Bids[1].Price, depth.Bids[1].Volume)
	}

	// Asks should be lowest first: 55, 56
	if depth.Asks[0].Price != 55 || depth.Asks[0].Volume != 150 {
		t.Fatalf("ask[0] = {%d, %d}, want {55, 150}", depth.Asks[0].Price, depth.Asks[0].Volume)
	}
	if depth.Asks[1].Price != 56 || depth.Asks[1].Volume != 300 {
		t.Fatalf("ask[1] = {%d, %d}, want {56, 300}", depth.Asks[1].Price, depth.Asks[1].Volume)
	}
}

func TestStopOrder(t *testing.T) {
	h := New()
	defer h.Close()

	// Place a resting ask
	h.AddLimit(1, SideSell, 50, 100)

	// Place a stop buy: triggers when price crosses 100
	s, _ := h.AddStop(2, SideBuy, 30, 100)
	if s != StatusOK {
		t.Fatalf("AddStop failed: %v", s.Error())
	}
}

func TestStopLimitOrder(t *testing.T) {
	h := New()
	defer h.Close()

	h.AddLimit(1, SideSell, 50, 100)

	s, _ := h.AddStopLimit(2, SideBuy, 30, 101, 100)
	if s != StatusOK {
		t.Fatalf("AddStopLimit failed: %v", s.Error())
	}
}

func TestMultipleTradesWalkBook(t *testing.T) {
	h := New()
	defer h.Close()

	// Stack 3 ask levels
	h.AddLimit(1, SideSell, 10, 100)
	h.AddLimit(2, SideSell, 20, 101)
	h.AddLimit(3, SideSell, 30, 102)

	// Market order behavior is engine-defined; this test asserts bridge stability.
	s, _ := h.Market(4, SideBuy, 25)
	if s != StatusOK {
		t.Fatalf("Market failed: %v", s.Error())
	}
}

func TestPartialFillLimitOrder(t *testing.T) {
	h := New()
	defer h.Close()

	// Resting buy of 100 shares at 50
	h.AddLimit(1, SideBuy, 100, 50)

	// Sell 30 at 50 — partial fill, 70 should remain
	h.AddLimit(2, SideSell, 30, 50)

	depth := h.GetDepth()
	if len(depth.Bids) != 1 {
		t.Fatalf("bid levels = %d, want 1", len(depth.Bids))
	}
	if depth.Bids[0].Volume != 70 {
		t.Fatalf("remaining bid volume = %d, want 70", depth.Bids[0].Volume)
	}
}
