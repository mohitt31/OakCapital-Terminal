package dbwriter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"synthbull/internal/eventbus"
)

// TradeRepository persists trades to PostgreSQL.
// Implements the handler callback for TradeWriter.
type TradeRepository struct {
	pool      *pgxpool.Pool
	symbolMap map[string]int64
}

// NewTradeRepository creates a TradeRepository.
//
// symbolMap: map from symbol string (e.g., "RELIANCE") to instrument_id.
// Typically built by db.SeedInstruments() during startup.
func NewTradeRepository(pool *pgxpool.Pool, symbolMap map[string]int64) *TradeRepository {
	return &TradeRepository{
		pool:      pool,
		symbolMap: symbolMap,
	}
}

// InsertTrade persists a single TradeEvent to the trades table.
// Called by TradeWriter for every matched trade.
//
// Returns error if:
//   - Symbol not in symbolMap (instrument_id not found)
//   - Database INSERT fails
//   - Price/Quantity cannot be converted to decimal
func (tr *TradeRepository) InsertTrade(ctx context.Context, ev eventbus.TradeEvent) error {
	instrumentID, ok := tr.symbolMap[ev.Symbol]
	if !ok {
		return fmt.Errorf("symbol %q not in symbol map", ev.Symbol)
	}

	// Convert int64 prices/quantities to decimal (cents → decimal dollars)
	price := decimal.New(ev.Price, -2)      // price in cents
	quantity := decimal.New(ev.Quantity, 0) // qty as-is

	_, err := tr.pool.Exec(ctx, `
		INSERT INTO trades_log (instrument_id, engine_buy_order_id, engine_sell_order_id, price, quantity,
		                   taker_side, executed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT DO NOTHING
	`,
		instrumentID,
		ev.MakerOrderID,
		ev.TakerOrderID,
		price,
		quantity,
		string(ev.TakerSide),
		ev.ExecutedAt,
	)
	if err != nil {
		return fmt.Errorf("insert trade: %w", err)
	}
	return nil
}

// InsertTradeHandler returns a handler function suitable for TradeWriter.Start()
func (tr *TradeRepository) InsertTradeHandler(ctx context.Context) func(eventbus.TradeEvent) error {
	return func(ev eventbus.TradeEvent) error {
		return tr.InsertTrade(ctx, ev)
	}
}

// ============================================================
// OrderRepository
// ============================================================

// OrderRepository persists order state changes to PostgreSQL.
// Per the dbwriter design, this writes full order snapshots on FILLED/PARTIALLY_FILLED.
type OrderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository creates an OrderRepository.
func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

// InsertOrderUpdate writes an order UPDATE record to order_history.
// Called by OrderWriter for terminal states (FILLED, PARTIALLY_FILLED).
//
// The order_history table is an append-only log of all order state changes.
// Callers can join on order_id + updated_at to reconstruct the full history.
func (or *OrderRepository) InsertOrderUpdate(ctx context.Context, ev eventbus.OrderUpdateEvent) error {
	_, err := or.pool.Exec(ctx, `
		INSERT INTO order_history (order_id, user_id, client_id, symbol, side, order_type,
		                           status, quantity, filled_qty, avg_price, limit_price, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		ev.OrderID,
		ev.UserID,
		ev.ClientID,
		ev.Symbol,
		string(ev.Side),
		string(ev.Type),
		string(ev.Status),
		ev.Quantity,
		ev.FilledQty,
		decimal.New(ev.AvgPrice, -2),
		decimal.New(ev.LimitPrice, -2),
		ev.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order update: %w", err)
	}
	return nil
}

// InsertOrderUpdateHandler returns a handler function suitable for OrderWriter.Start()
func (or *OrderRepository) InsertOrderUpdateHandler(ctx context.Context) func(eventbus.OrderUpdateEvent) error {
	return func(ev eventbus.OrderUpdateEvent) error {
		return or.InsertOrderUpdate(ctx, ev)
	}
}

// ============================================================
// CandleRepository (already mostly in place, extending here)
// ============================================================

// CandleRepository already exists in internal/db/candle_repo.go
// This is a hint to extend it if needed.
//
// Example batch insert:
//
//	func (cr *CandleRepository) InsertBatch(ctx context.Context, batch []eventbus.CandleEvent) error {
//	    // Use pgx.CopyFrom for bulk insert efficiency
//	}

// ============================================================
// ErrorRepository
// ============================================================

// ErrorRepository persists system errors to PostgreSQL.
type ErrorRepository struct {
	pool *pgxpool.Pool
}

// NewErrorRepository creates an ErrorRepository.
func NewErrorRepository(pool *pgxpool.Pool) *ErrorRepository {
	return &ErrorRepository{pool: pool}
}

// InsertError writes an ErrorEvent to system_event_log.
func (er *ErrorRepository) InsertError(ctx context.Context, ev eventbus.ErrorEvent) error {
	payload, _ := json.Marshal(ev)
	_, err := er.pool.Exec(ctx, `
		INSERT INTO system_event_log (event_type, severity, payload, created_at)
		VALUES ($1, $2, $3, $4)
	`,
		"ERROR_"+ev.ErrorCode,
		ev.Severity,
		payload,
		ev.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert error event: %w", err)
	}
	return nil
}

// InsertErrorHandler returns a handler function suitable for ErrorWriter.Start()
func (er *ErrorRepository) InsertErrorHandler(ctx context.Context) func(eventbus.ErrorEvent) error {
	return func(ev eventbus.ErrorEvent) error {
		return er.InsertError(ctx, ev)
	}
}

// ============================================================
// AlertRepository
// ============================================================

// AlertRepository persists user alerts to PostgreSQL.
type AlertRepository struct {
	pool *pgxpool.Pool
}

// NewAlertRepository creates an AlertRepository.
func NewAlertRepository(pool *pgxpool.Pool) *AlertRepository {
	return &AlertRepository{pool: pool}
}

// InsertAlert writes an AlertEvent to user_alerts table.
//
// Alert types: price_threshold, portfolio_milestone, order_status, system_notification, etc.
func (ar *AlertRepository) InsertAlert(ctx context.Context, ev eventbus.AlertEvent) error {
	payload, _ := json.Marshal(ev)
	_, err := ar.pool.Exec(ctx, `
		INSERT INTO user_alerts (user_id, alert_type, symbol, message, severity, payload, created_at, is_read)
		VALUES ($1, $2, $3, $4, $5, $6, $7, false)
	`,
		ev.UserID,
		ev.AlertType,
		ev.Symbol,
		ev.Message,
		ev.Severity,
		payload,
		ev.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert alert: %w", err)
	}
	return nil
}

// InsertAlertHandler returns a handler function suitable for AlertWriter.Start()
func (ar *AlertRepository) InsertAlertHandler(ctx context.Context) func(eventbus.AlertEvent) error {
	return func(ev eventbus.AlertEvent) error {
		return ar.InsertAlert(ctx, ev)
	}
}

// ============================================================
// BotStatusRepository
// ============================================================

// BotStatusRepository persists bot lifecycle events to PostgreSQL.
type BotStatusRepository struct {
	pool *pgxpool.Pool
}

// NewBotStatusRepository creates a BotStatusRepository.
func NewBotStatusRepository(pool *pgxpool.Pool) *BotStatusRepository {
	return &BotStatusRepository{pool: pool}
}

// InsertBotStatus writes a BotStatusEvent to bot_status_log.
// Used for auditing and debugging bot behavior over time.
func (br *BotStatusRepository) InsertBotStatus(ctx context.Context, ev eventbus.BotStatusEvent) error {
	payload, _ := json.Marshal(ev)
	_, err := br.pool.Exec(ctx, `
		INSERT INTO bot_status_log (bot_id, client_id, user_id, status, message, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		ev.BotID,
		ev.ClientID,
		ev.UserID,
		ev.Status,
		ev.Message,
		payload,
		ev.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert bot status: %w", err)
	}
	return nil
}

// InsertBotStatusHandler returns a handler function suitable for BotStatusWriter
func (br *BotStatusRepository) InsertBotStatusHandler(ctx context.Context) func(eventbus.BotStatusEvent) error {
	return func(ev eventbus.BotStatusEvent) error {
		return br.InsertBotStatus(ctx, ev)
	}
}

// ============================================================
// PortfolioSnapshotRepository
// ============================================================

// PortfolioSnapshotRepository stores periodic portfolio snapshots for auditing.
// (Note: The real-time portfolio state is managed by internal/portfolio/manager.go)
type PortfolioSnapshotRepository struct {
	pool *pgxpool.Pool
}

// NewPortfolioSnapshotRepository creates a PortfolioSnapshotRepository.
func NewPortfolioSnapshotRepository(pool *pgxpool.Pool) *PortfolioSnapshotRepository {
	return &PortfolioSnapshotRepository{pool: pool}
}

// InsertPortfolioSnapshot writes a portfolio state snapshot to the database.
// Useful for historical analysis, P&L tracking, and debugging.
func (pr *PortfolioSnapshotRepository) InsertPortfolioSnapshot(ctx context.Context, ev eventbus.PortfolioEvent) error {
	payload, _ := json.Marshal(ev)
	positions := decimal.New(int64(len(ev.Positions)), 0)

	_, err := pr.pool.Exec(ctx, `
		INSERT INTO portfolio_snapshots (user_id, total_cash, realized_pnl, unrealized_pnl,
		                                  num_positions, payload, snapshot_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		ev.UserID,
		decimal.New(ev.Cash, -2),
		decimal.New(0, 0),       // realized PnL (not in event)
		decimal.New(ev.PnL, -2), // unrealized PnL
		positions,
		payload,
		ev.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert portfolio snapshot: %w", err)
	}
	return nil
}

// InsertPortfolioSnapshotHandler returns a handler function for PortfolioWriter
func (pr *PortfolioSnapshotRepository) InsertPortfolioSnapshotHandler(ctx context.Context) func(eventbus.PortfolioEvent) error {
	return func(ev eventbus.PortfolioEvent) error {
		return pr.InsertPortfolioSnapshot(ctx, ev)
	}
}

// ============================================================
// HealthEventRepository
// ============================================================

// HealthEventRepository logs system health events for monitoring.
type HealthEventRepository struct {
	pool *pgxpool.Pool
}

// NewHealthEventRepository creates a HealthEventRepository.
func NewHealthEventRepository(pool *pgxpool.Pool) *HealthEventRepository {
	return &HealthEventRepository{pool: pool}
}

// InsertHealthEvent writes a HealthEvent to system_event_log.
// Used for uptime tracking and service health monitoring.
func (hr *HealthEventRepository) InsertHealthEvent(ctx context.Context, ev eventbus.HealthEvent) error {
	payload, _ := json.Marshal(ev)
	_, err := hr.pool.Exec(ctx, `
		INSERT INTO system_event_log (event_type, severity, payload, created_at)
		VALUES ($1, $2, $3, $4)
	`,
		"HEALTH_"+ev.ServiceName,
		"INFO",
		payload,
		ev.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert health event: %w", err)
	}
	return nil
}

// ============================================================
// Batch Operations
// ============================================================

// BatchTradeInsert writes multiple trades in a single batch operation.
// More efficient than individual inserts for historical data backfill.
func (tr *TradeRepository) BatchTradeInsert(ctx context.Context, trades []eventbus.TradeEvent) error {
	if len(trades) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, t := range trades {
		instrumentID, ok := tr.symbolMap[t.Symbol]
		if !ok {
			// Skip trades for unknown symbols
			continue
		}
		price := decimal.New(t.Price, -2)
		quantity := decimal.New(t.Quantity, 0)

		batch.Queue(`
			INSERT INTO trades_log (instrument_id, engine_buy_order_id, engine_sell_order_id, price, quantity,
			                   taker_side, executed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT DO NOTHING
		`,
			instrumentID, t.MakerOrderID, t.TakerOrderID, price, quantity,
			string(t.TakerSide), t.ExecutedAt,
		)
	}

	results := tr.pool.SendBatch(ctx, batch)
	defer results.Close()

	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("batch trade insert at index %d: %w", i, err)
		}
	}
	return nil
}
