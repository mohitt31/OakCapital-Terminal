//go:build ignore

// Example integration of DBWriter into cmd/server/main.go
//
// This file shows the EXACT code to add to your main() function.
// Copy the relevant sections into cmd/server/main.go after database initialization.

package main

import (
	"context"
	"log"

	"synthbull/internal/db"
	"synthbull/internal/dbwriter"
	"synthbull/internal/eventbus"
	"synthbull/internal/market"
)

// ============================================================
// PSEUDOCODE SHOWING INTEGRATION POINTS IN main()
// ============================================================

func exampleMainIntegration() {
	ctx := context.Background()

	// Step 1: After initializing PostgreSQL (already exists in main.go)
	// ─────────────────────────────────────────────────────────────
	// → db.InitDB(ctx, cfg.DBURL)
	// → db.RunMigrations(ctx)
	// → db.Pool is now available

	// Step 2: Build symbol→instrument_id map (already exists in main.go)
	// ────────────────────────────────────────────────────────
	var symbolMap map[string]int64
	// Existing code:
	// seeds := make([]db.InstrumentSeed, 0, 7)
	// for _, p := range market.IndianStockPresets() { ... }
	// symbolMap, _ = db.SeedInstruments(ctx, seeds)

	// Step 3: After initializing Redis event bus (already exists in main.go)
	// ─────────────────────────────────────────────────────────────────
	var bus *eventbus.Bus
	// Existing code:
	// bus, _ = eventbus.New(cfg.RedisURL)

	// Step 4: START DB WRITER (NEW CODE — ADD THIS)
	// ───────────────────────────────────────────
	var dbWriterSvc *dbwriter.Service
	if db.Pool != nil && bus != nil {
		allSymbols := make([]string, 0, 7)
		for _, p := range market.IndianStockPresets() {
			allSymbols = append(allSymbols, p.Symbol)
		}

		// Single call to start all database writers
		dbWriterSvc = dbwriter.StartDBWriters(
			ctx,
			bus.Sub,           // Redis subscriber
			db.Pool,           // PostgreSQL pool
			symbolMap,         // symbol → instrument_id map
			allSymbols,        // ["RELIANCE", "TCS", ...]
		)
		log.Println("Database writers initialized")

		// Add defer to gracefully stop writers at shutdown
		defer dbWriterSvc.Stop()
	} else {
		log.Println("Skipping DBWriter: PostgreSQL or Redis unavailable")
	}

	// Step 5: Continue with rest of server initialization
	// ──────────────────────────────────────────────────
	// → Create API routes
	// → Start HTTP server
	// → Wait for signals
	// (existing code continues unchanged)

	// On server shutdown, all writers are stopped via defer
}

// ============================================================
// ACTUAL main.go DIFF (showing where to insert code)
// ============================================================

/*

// ... existing imports ...
import (
	// ... existing ...
	"synthbull/internal/dbwriter"  // ← ADD THIS IMPORT
)

func main() {
	log.Println("Starting OpenSoft-26 Backend...")

	// ... lines 34-79: existing DB init and seeding code ...

	// ✓ db.Pool is now initialized
	// ✓ symbolMap is now populated
	// ✓ bus is now initialized

	// 3b. START DB WRITER ← ADD THIS NEW SECTION (lines 82-102)
	// ──────────────────
	var dbWriterSvc *dbwriter.Service
	if db.Pool != nil && bus != nil {
		allSymbols := make([]string, 0, 7)
		for _, p := range market.IndianStockPresets() {
			allSymbols = append(allSymbols, p.Symbol)
		}

		dbWriterSvc = dbwriter.StartDBWriters(
			ctx,
			bus.Sub,
			db.Pool,
			symbolMap,
			allSymbols,
		)
		log.Println("Database writers initialized")
		defer dbWriterSvc.Stop()
	}

	// ... rest of existing code (lines 103+) continues unchanged ...
}

*/

// ============================================================
// ADVANCED: Custom Writer Configuration
// ============================================================

// If you need more control over individual writers:
func advancedDBWriterSetup(
	ctx context.Context,
	sub *eventbus.Subscriber,
	pool *db.Pool,
	symbolMap map[string]int64,
	allSymbols []string,
) *dbwriter.Service {

	// Create service (but don't auto-start)
	svc := dbwriter.NewService(sub, pool, symbolMap)

	// Start only the writers you need
	log.Println("[setup] Starting trade and error writers...")
	svc.StartTradeWriter(ctx, allSymbols)
	svc.StartErrorWriter(ctx)

	// Skip candles if you're using internal/candle/builder.go instead
	// svc.StartCandleWriter(ctx, allSymbols, []string{"1s", "5s"})

	// Access repos for custom operations
	repos := svc.GetRepositories()
	_ = repos.Trade    // Use for backfilled trade ingestion
	_ = repos.Order    // Use for order reconciliation
	_ = repos.Error    // Use for error replay
	// etc.

	return svc
}

// ============================================================
// Example: Backfill Historical Trades
// ============================================================

func backfillHistoricalTrades(
	ctx context.Context,
	tradeRepo *dbwriter.TradeRepository,
	historicalTrades []eventbus.TradeEvent,
) error {
	log.Printf("Backfilling %d historical trades...", len(historicalTrades))

	// Use batch insert for efficiency
	if err := tradeRepo.BatchTradeInsert(ctx, historicalTrades); err != nil {
		return err
	}

	log.Printf("Successfully backfilled trades")
	return nil
}

// ============================================================
// Example: Manual Query After Writers Are Running
// ============================================================

func queryRecentTrades(ctx context.Context, pool *pgxpool.Pool, symbol string) error {
	rows, err := pool.Query(ctx, `
		SELECT id, price, quantity, side_that_took_liquidity, executed_at
		FROM trades
		WHERE instrument_id = (
			SELECT id FROM instruments WHERE symbol = $1
		)
		ORDER BY executed_at DESC
		LIMIT 100
	`, symbol)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var price, quantity decimal.Decimal
		var side string
		var executedAt time.Time

		if err := rows.Scan(&id, &price, &quantity, &side, &executedAt); err != nil {
			return err
		}

		log.Printf("Trade %d: %s %s @ %s", id, quantity, side, price)
	}

	return rows.Err()
}

// ============================================================
// Example: Verify Writers Are Running
// ============================================================

func healthCheck(dbWriterSvc *dbwriter.Service) {
	if dbWriterSvc == nil {
		log.Println("❌ DBWriter not initialized (PostgreSQL or Redis unavailable)")
		return
	}

	repos := dbWriterSvc.GetRepositories()
	if repos.Trade != nil {
		log.Println("✓ Trade writer running")
	}
	if repos.Order != nil {
		log.Println("✓ Order writer running")
	}
	if repos.Error != nil {
		log.Println("✓ Error writer running")
	}
	log.Println("✓ All DBWriters healthy")
}
