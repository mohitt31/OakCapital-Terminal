package dbwriter

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"synthbull/internal/eventbus"
)

// Service encapsulates all database writer components for the backend.
// Manages initialization, startup, and graceful shutdown of all persistent storage consumers.
type Service struct {
	candleWriter    *CandleWriter
	orderWriter     *OrderWriter
	tradeWriter     *TradeWriter
	errorWriter     *ErrorWriter
	alertWriter     *AlertWriter
	botStatusWriter *BotStatusWriter
	portfolioWriter *PortfolioWriter

	// Repositories handle actual database operations
	tradeRepo       *TradeRepository
	orderRepo       *OrderRepository
	errorRepo       *ErrorRepository
	alertRepo       *AlertRepository
	botStatusRepo   *BotStatusRepository
	portfolioRepo   *PortfolioSnapshotRepository

	subscriber *eventbus.Subscriber
}

// NewService creates a fully wired DBWriter Service.
//
// This is the main entry point for integrating all database persistence into your backend.
//
// Example usage in main.go:
//
//	// After initializing Redis event bus and DB pool:
//	dbWriterSvc := dbwriter.NewService(bus.Sub, db.Pool, symbolMap)
//	dbWriterSvc.Start(ctx, allSymbols)
//	defer dbWriterSvc.Stop()
func NewService(sub *eventbus.Subscriber, pool *pgxpool.Pool, symbolMap map[string]int64) *Service {
	return &Service{
		candleWriter:    NewCandleWriter(sub, 60),
		orderWriter:     NewOrderWriter(sub),
		tradeWriter:     NewTradeWriter(sub),
		errorWriter:     NewErrorWriter(sub),
		alertWriter:     NewAlertWriter(sub),
		botStatusWriter: NewBotStatusWriter(sub),
		portfolioWriter: NewPortfolioWriter(sub),

		tradeRepo:       NewTradeRepository(pool, symbolMap),
		orderRepo:       NewOrderRepository(pool),
		errorRepo:       NewErrorRepository(pool),
		alertRepo:       NewAlertRepository(pool),
		botStatusRepo:   NewBotStatusRepository(pool),
		portfolioRepo:   NewPortfolioSnapshotRepository(pool),

		subscriber: sub,
	}
}

// Start launches all database writers with their respective handlers.
// This should be called once during application startup.
//
// Parameters:
//   - ctx: background context for goroutine lifecycle
//   - symbols: list of trading instruments (e.g., ["RELIANCE", "TCS", "INFY"])
//
// All writers run concurrently in background goroutines.
// Call Stop() to gracefully shut them down.
func (s *Service) Start(ctx context.Context, symbols []string) {
	log.Println("[dbwriter] initializing all writers...")

	// Start Candle Writer (batched)
	s.candleWriter.Start(ctx, symbols, []string{"1s", "5s"}, func(batch []eventbus.CandleEvent) error {
		// Batch candle inserts for efficiency
		// You could also implement a batch method in the candle repo
		return s.insertCandlesBatch(ctx, batch)
	})

	// Start Order Writer
	s.orderWriter.Start(ctx, s.orderRepo.InsertOrderUpdateHandler(ctx))

	// Start Trade Writer
	s.tradeWriter.Start(ctx, symbols, s.tradeRepo.InsertTradeHandler(ctx))

	// Start Error Writer
	s.errorWriter.Start(ctx, s.errorRepo.InsertErrorHandler(ctx))

	// Optional: Start Alert Writer (requires pub/sub model or global stream)
	// s.alertWriter.StartGlobal(ctx, s.alertRepo.InsertAlertHandler(ctx))

	// Optional: Start Bot Status Writer (requires per-bot subscription)
	// s.botStatusWriter.StartGlobal(ctx, s.botStatusRepo.InsertBotStatusHandler(ctx))

	log.Println("[dbwriter] all writers started successfully")
}

// Stop gracefully shuts down all running writers.
// Blocks until all consumer goroutines finish.
//
// This is typically called in a defer statement after Start():
//
//	defer dbWriterSvc.Stop()
func (s *Service) Stop() {
	log.Println("[dbwriter] stopping all writers...")
	s.subscriber.Wait()
	log.Println("[dbwriter] all writers stopped")
}

// insertCandlesBatch is a helper that efficiently inserts a batch of candles.
// In the future, this could be moved to an optimized candle repository method.
func (s *Service) insertCandlesBatch(ctx context.Context, batch []eventbus.CandleEvent) error {
	if len(batch) == 0 {
		return nil
	}

	// For now, this is a placeholder.
	// The actual candle insertion is handled by internal/candle/builder.go → CandleRepo.
	// You can extend CandleRepo.InsertBatch() if needed for direct stream consumption.

	log.Printf("[dbwriter] processing %d candles", len(batch))
	return nil
}

// ============================================================
// Integration Helper: Call from main.go
// ============================================================

// StartDBWriters is the recommended way to initialize all database writers in main.go.
//
// Example main.go snippet:
//
//	// After initializing PostgreSQL pool and Redis event bus:
//	if db.Pool != nil && bus != nil {
//	    dbWriterSvc := dbwriter.StartDBWriters(ctx, bus.Sub, db.Pool, symbolMap, allSymbols)
//	    defer dbWriterSvc.Stop()
//	}
//
// This is intentionally a module-level function to make integration clear and discoverable.
func StartDBWriters(ctx context.Context, sub *eventbus.Subscriber, pool *pgxpool.Pool, symbolMap map[string]int64, symbols []string) *Service {
	if sub == nil || pool == nil {
		log.Println("[dbwriter] skipping: Redis or PostgreSQL unavailable")
		return nil
	}

	svc := NewService(sub, pool, symbolMap)
	svc.Start(ctx, symbols)
	return svc
}

// ============================================================
// Manual Per-Writer Control (advanced usage)
// ============================================================

// For more granular control, callers can start individual writers:

// StartCandleWriter starts only the candle writer.
func (s *Service) StartCandleWriter(ctx context.Context, symbols []string, intervals []string) {
	s.candleWriter.Start(ctx, symbols, intervals, func(batch []eventbus.CandleEvent) error {
		return s.insertCandlesBatch(ctx, batch)
	})
}

// StartOrderWriter starts only the order writer.
func (s *Service) StartOrderWriter(ctx context.Context) {
	s.orderWriter.Start(ctx, s.orderRepo.InsertOrderUpdateHandler(ctx))
}

// StartTradeWriter starts only the trade writer.
func (s *Service) StartTradeWriter(ctx context.Context, symbols []string) {
	s.tradeWriter.Start(ctx, symbols, s.tradeRepo.InsertTradeHandler(ctx))
}

// StartErrorWriter starts only the error writer.
func (s *Service) StartErrorWriter(ctx context.Context) {
	s.errorWriter.Start(ctx, s.errorRepo.InsertErrorHandler(ctx))
}

// GetRepositories returns all repository instances for direct use if needed.
// Useful for running manual queries or batch operations outside the event stream.
func (s *Service) GetRepositories() struct {
	Trade       *TradeRepository
	Order       *OrderRepository
	Error       *ErrorRepository
	Alert       *AlertRepository
	BotStatus   *BotStatusRepository
	Portfolio   *PortfolioSnapshotRepository
} {
	return struct {
		Trade       *TradeRepository
		Order       *OrderRepository
		Error       *ErrorRepository
		Alert       *AlertRepository
		BotStatus   *BotStatusRepository
		Portfolio   *PortfolioSnapshotRepository
	}{
		Trade:       s.tradeRepo,
		Order:       s.orderRepo,
		Error:       s.errorRepo,
		Alert:       s.alertRepo,
		BotStatus:   s.botStatusRepo,
		Portfolio:   s.portfolioRepo,
	}
}
