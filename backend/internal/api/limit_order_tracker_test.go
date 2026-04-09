package api

import (
	"context"
	"math"
	"testing"

	"synthbull/internal/engine"
	"synthbull/pkg/models"
)

func TestLimitOrderTracker_RecordPlacementTrades_PartialBuyFill(t *testing.T) {
	tracker := NewLimitOrderTracker()
	tracker.Track("u1", "reliance", models.SideBuy, 101, 10, 10000)

	release, completed := tracker.RecordPlacementTrades("RELIANCE", 101, []engine.Trade{
		{MakerOrderID: 101, Price: 9900, Qty: 4},
	})
	if completed {
		t.Fatalf("expected order to remain open after partial fill")
	}
	if release != 0 {
		t.Fatalf("expected no reserve release on partial fill, got %.4f", release)
	}

	snap, ok := tracker.Remove("Reliance", 101)
	if !ok {
		t.Fatalf("expected tracked order to exist")
	}
	if snap.FilledQty != 4 {
		t.Fatalf("filled qty = %d, want 4", snap.FilledQty)
	}

	wantRemaining := 1000.0 - 396.0
	if math.Abs(snap.RemainingReserve-wantRemaining) > 1e-9 {
		t.Fatalf("remaining reserve = %.4f, want %.4f", snap.RemainingReserve, wantRemaining)
	}
}

func TestLimitOrderTracker_RecordPlacementTrades_FullBuyFillReleasesReserve(t *testing.T) {
	tracker := NewLimitOrderTracker()
	tracker.Track("u1", "TCS", models.SideBuy, 7, 5, 10000)

	release, completed := tracker.RecordPlacementTrades("tcs", 7, []engine.Trade{
		{MakerOrderID: 7, Price: 9800, Qty: 5},
	})
	if !completed {
		t.Fatalf("expected order to complete")
	}

	wantRelease := 10.0 // reserved 500.00, used 490.00
	if math.Abs(release-wantRelease) > 1e-9 {
		t.Fatalf("release reserve = %.4f, want %.4f", release, wantRelease)
	}

	if _, ok := tracker.Remove("TCS", 7); ok {
		t.Fatalf("expected completed order to be removed from tracker")
	}
}

func TestLimitOrderTracker_ReconcileTrades_PartialSellFill(t *testing.T) {
	tracker := NewLimitOrderTracker()
	tracker.Track("u2", "HDFCBANK", models.SideSell, 55, 8, 12000)

	tracker.ReconcileTrades(context.Background(), "HDFCBANK", []engine.Trade{
		{MakerOrderID: 55, Price: 11950, Qty: 3},
	}, nil, nil)

	snap, ok := tracker.Remove("hdfcbank", 55)
	if !ok {
		t.Fatalf("expected tracked sell order to exist after partial fill")
	}
	if snap.FilledQty != 3 {
		t.Fatalf("filled qty = %d, want 3", snap.FilledQty)
	}
	if snap.RemainingReserve != 0 {
		t.Fatalf("sell order should not keep cash reserve, got %.4f", snap.RemainingReserve)
	}
}

func TestLimitOrderTracker_ReconcileTrades_FullBuyFillRemovesOrder(t *testing.T) {
	tracker := NewLimitOrderTracker()
	tracker.Track("u3", "SBIN", models.SideBuy, 88, 2, 10000)

	tracker.ReconcileTrades(context.Background(), "SBIN", []engine.Trade{
		{MakerOrderID: 88, Price: 9950, Qty: 2},
	}, nil, nil)

	if _, ok := tracker.Remove("sbin", 88); ok {
		t.Fatalf("expected full fill reconciliation to remove order")
	}
}

func TestLimitOrderTracker_UpdateLimit(t *testing.T) {
	tracker := NewLimitOrderTracker()
	tracker.Track("u4", "ICICIBANK", models.SideSell, 99, 4, 11000)
	tracker.UpdateLimit("icicibank", 99, 9, 11100)

	snap, ok := tracker.Remove("ICICIBANK", 99)
	if !ok {
		t.Fatalf("expected tracked order to exist")
	}
	if snap.Quantity != 9 {
		t.Fatalf("quantity = %d, want 9", snap.Quantity)
	}
}
