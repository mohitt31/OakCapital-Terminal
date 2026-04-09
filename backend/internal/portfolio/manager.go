package portfolio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"synthbull/internal/api/ws"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

// Manager is the central portfolio service. One instance per server.
// It maintains an in-memory map of userID -> Portfolio and periodically
// flushes to PostgreSQL.
type Manager struct {
	mu      sync.RWMutex
	store   map[string]*Portfolio // userID -> portfolio
	db      *pgxpool.Pool
	flushCh chan string // userIDs that need a DB flush
	stopCh  chan struct{}
	wsHub   Broadcaster // WebSocket hub for broadcasting updates
}

// Broadcaster defines the interface for broadcasting user-specific messages.
type Broadcaster interface {
	BroadcastToUser(userID string, msg interface{})
}

// NewManager creates and starts the portfolio manager.
func NewManager(db *pgxpool.Pool, wsHub Broadcaster) *Manager {
	m := &Manager{
		store:   make(map[string]*Portfolio),
		db:      db,
		flushCh: make(chan string, 1024),
		stopCh:  make(chan struct{}),
		wsHub:   wsHub,
	}
	go m.flushWorker()
	return m
}

// Stop shuts down the flush goroutine.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// ─────────────────────────────────────────────
//  Public API — called by HTTP handlers
// ─────────────────────────────────────────────

// LoadOrFetch returns the in-memory portfolio for a user, loading from DB
// if it isn't cached yet.
func (m *Manager) LoadOrFetch(ctx context.Context, userID string) (*Portfolio, error) {
	m.mu.RLock()
	p, ok := m.store[userID]
	m.mu.RUnlock()
	if ok {
		return p, nil
	}
	return m.fetchFromDB(ctx, userID)
}

// GetSnapshot returns a JSON-safe snapshot of the portfolio with live P&L.
func (m *Manager) GetSnapshot(ctx context.Context, userID string) (*Snapshot, error) {
	p, err := m.LoadOrFetch(ctx, userID)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return buildSnapshot(p), nil
}

// ─────────────────────────────────────────────
//  Order lifecycle hooks — called by order API
// ─────────────────────────────────────────────

// ReserveCash is called when a buy limit order is placed.
// It moves (price * qty) from available to blocked (escrow).
func (m *Manager) ReserveCash(ctx context.Context, userID string, price, qty float64) error {
	amount := price * qty
	m.mu.Lock()
	p, err := m.getOrFetchLocked(ctx, userID)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	if p.AvailableCash < amount {
		m.mu.Unlock()
		return fmt.Errorf("insufficient cash: have %.2f, need %.2f", p.AvailableCash, amount)
	}
	p.AvailableCash -= amount
	p.BlockedCash += amount
	p.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.scheduleFLush(userID)
	m.broadcastUpdate(userID) // Broadcast portfolio update
	return nil
}

// ReleaseCash is called when a buy limit order is cancelled.
// It moves back from blocked to available.
func (m *Manager) ReleaseCash(ctx context.Context, userID string, price, qty float64) {
	m.ReleaseCashAmount(ctx, userID, price*qty)
}

// ReleaseCashAmount releases an explicit cash amount from blocked to available.
func (m *Manager) ReleaseCashAmount(ctx context.Context, userID string, amount float64) {
	if amount <= 0 {
		return
	}
	m.mu.Lock()
	if p, ok := m.store[userID]; ok {
		release := min(amount, p.BlockedCash)
		p.BlockedCash -= release
		p.AvailableCash += release
		p.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	m.scheduleFLush(userID)
	m.broadcastUpdate(userID) // Broadcast portfolio update
}

// ApplyFill is the most important method. Called after the matching engine
// reports a trade. It:
//  1. Deducts cash for buys (from blocked for limits, from available for markets).
//  2. Adds cash for sells.
//  3. Updates positions using weighted-average cost.
//  4. Calculates realised P&L for closes.
//  5. Updates mark price and unrealised P&L.
//
// Returns a Fill record suitable for broadcasting over WS.
func (m *Manager) ApplyFill(ctx context.Context, userID, symbol, side string, price, qty float64, isLimit bool) (*Fill, error) {
	m.mu.Lock()

	p, err := m.getOrFetchLocked(ctx, userID)
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}

	cashDelta := price * qty
	var realisedPnL float64

	if side == "buy" {
		if isLimit {
			// Release from escrow
			release := min(cashDelta, p.BlockedCash)
			p.BlockedCash -= release
		} else {
			// Market order — deduct from available directly
			if p.AvailableCash < cashDelta {
				m.mu.Unlock()
				return nil, fmt.Errorf("insufficient available cash for market buy")
			}
			p.AvailableCash -= cashDelta
		}
		p.TotalCash -= cashDelta

		// Update position (long)
		pos := m.getOrCreatePosition(p, symbol)
		if pos.Quantity >= 0 {
			// Adding to long
			pos.AvgEntry = ((pos.Quantity * pos.AvgEntry) + (qty * price)) / (pos.Quantity + qty)
		} else {
			// Closing short
			realisedPnL = (pos.AvgEntry - price) * qty
			if pos.Quantity+qty >= 0 {
				pos.AvgEntry = price
			}
		}
		pos.Quantity += qty
		pos.MarkPrice = price
		pos.PnL = (price - pos.AvgEntry) * pos.Quantity

	} else { // sell
		p.TotalCash += cashDelta
		p.AvailableCash += cashDelta

		pos := m.getOrCreatePosition(p, symbol)
		if pos.Quantity <= 0 {
			// Adding to short
			pos.AvgEntry = ((math_abs(pos.Quantity)*pos.AvgEntry + qty*price) / (math_abs(pos.Quantity) + qty))
		} else {
			// Closing long
			realisedPnL = (price - pos.AvgEntry) * qty
			if pos.Quantity-qty <= 0 {
				pos.AvgEntry = price
			}
		}
		pos.Quantity -= qty
		pos.MarkPrice = price
		pos.PnL = (price - pos.AvgEntry) * pos.Quantity
	}

	// Remove zero positions
	for sym, pos := range p.Positions {
		if math_abs(pos.Quantity) < 1e-9 {
			delete(p.Positions, sym)
		}
	}

	p.UpdatedAt = time.Now()
	m.mu.Unlock()

	// Broadcast and flush AFTER releasing the lock to avoid deadlock.
	m.scheduleFLush(userID)
	m.broadcastUpdate(userID)

	return &Fill{
		Symbol:    symbol,
		Side:      side,
		Price:     price,
		Qty:       qty,
		PnL:       realisedPnL,
		Timestamp: time.Now(),
	}, nil
}

// UpdateMarkPrice refreshes the unrealised P&L for all positions in a symbol
// based on the latest trade price from the engine.
func (m *Manager) UpdateMarkPrice(userID, symbol string, lastPrice float64) {
	m.mu.Lock()
	p, ok := m.store[userID]
	if !ok {
		m.mu.Unlock()
		return
	}
	pos, ok := p.Positions[symbol]
	if !ok {
		m.mu.Unlock()
		return
	}
	pos.MarkPrice = lastPrice
	pos.PnL = (lastPrice - pos.AvgEntry) * pos.Quantity
	m.mu.Unlock()
	// Broadcast AFTER releasing the lock to avoid deadlock (broadcastUpdate acquires RLock).
	m.broadcastUpdate(userID)
}

// UpdateAllMarkPrices updates the mark price for every user who holds the symbol.
func (m *Manager) UpdateAllMarkPrices(symbol string, lastPrice float64) {
	m.mu.Lock()
	// Collect affected userIDs under the write lock, then broadcast after releasing.
	var affectedUsers []string
	for _, p := range m.store {
		if pos, ok := p.Positions[symbol]; ok {
			pos.MarkPrice = lastPrice
			pos.PnL = (lastPrice - pos.AvgEntry) * pos.Quantity
			affectedUsers = append(affectedUsers, p.UserID)
		}
	}
	m.mu.Unlock()

	for _, uid := range affectedUsers {
		m.broadcastUpdate(uid)
	}
}

// ─────────────────────────────────────────────
//  Internal helpers
// ─────────────────────────────────────────────

func (m *Manager) getOrCreatePosition(p *Portfolio, symbol string) *Position {
	if pos, ok := p.Positions[symbol]; ok {
		return pos
	}
	pos := &Position{Symbol: symbol}
	p.Positions[symbol] = pos
	return pos
}

// getOrFetchLocked loads portfolio under the lock. Caller MUST hold m.mu.Lock().
func (m *Manager) getOrFetchLocked(ctx context.Context, userID string) (*Portfolio, error) {
	if p, ok := m.store[userID]; ok {
		return p, nil
	}
	// Important: Release the lock before calling fetchFromDB to avoid blocking other users
	// while waiting for Postgres. fetchFromDB will re-acquire the lock to update m.store.
	m.mu.Unlock()
	p, err := m.fetchFromDB(ctx, userID)
	m.mu.Lock()

	// If we successfully fetched but m.store was updated by another goroutine,
	// return the one in the store to ensure consistency.
	if err == nil {
		if existing, ok := m.store[userID]; ok {
			return existing, nil
		}
		// This should not happen since fetchFromDB updates the store.
	}
	return p, err
}

func (m *Manager) fetchFromDB(ctx context.Context, userID string) (*Portfolio, error) {
	if m.db == nil {
		// No database mode: keep portfolio simulation functional in-memory.
		p := &Portfolio{
			UserID:        userID,
			PortfolioID:   0,
			TotalCash:     100000.0,
			AvailableCash: 100000.0,
			BlockedCash:   0,
			Positions:     make(map[string]*Position),
			UpdatedAt:     time.Now(),
		}
		m.mu.Lock()
		m.store[userID] = p
		m.mu.Unlock()
		return p, nil
	}

	row := m.db.QueryRow(ctx,
		`SELECT id, total_cash, available_cash, blocked_cash FROM portfolios WHERE user_id = $1 LIMIT 1`,
		userID,
	)
	var portfolioID int64
	var totalCash, availableCash, blockedCash float64
	if err := row.Scan(&portfolioID, &totalCash, &availableCash, &blockedCash); err != nil {
		// Legacy users (or partially seeded DBs) may miss a portfolio row.
		// Auto-initialize instead of returning 500 to clients.
		if err == pgx.ErrNoRows {
			exists, userErr := m.userExists(ctx, userID)
			if userErr != nil {
				return nil, fmt.Errorf("failed to verify user %s: %w", userID, userErr)
			}
			if !exists {
				return nil, ErrUserNotFound
			}
			createdID, createdTotal, createdAvailable, createdBlocked, createErr := m.ensureDefaultPortfolioRow(ctx, userID)
			if createErr != nil {
				return nil, fmt.Errorf("failed to initialize portfolio for user %s: %w", userID, createErr)
			}
			portfolioID = createdID
			totalCash = createdTotal
			availableCash = createdAvailable
			blockedCash = createdBlocked
		} else {
			return nil, fmt.Errorf("failed to load portfolio for user %s: %w", userID, err)
		}
	}

	// Load positions
	rows, err := m.db.Query(ctx, `
		SELECT i.symbol, pos.net_quantity, pos.average_entry_price
		FROM positions pos
		JOIN instruments i ON i.id = pos.instrument_id
		JOIN portfolios pf ON pf.id = pos.portfolio_id
		WHERE pf.user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load positions: %w", err)
	}
	defer rows.Close()

	positions := make(map[string]*Position)
	for rows.Next() {
		var sym string
		var qty, avgEntry float64
		if err := rows.Scan(&sym, &qty, &avgEntry); err != nil {
			continue
		}
		positions[sym] = &Position{
			Symbol:    sym,
			Quantity:  qty,
			AvgEntry:  avgEntry,
			MarkPrice: avgEntry, // will be refreshed on next tick
		}
	}

	p := &Portfolio{
		UserID:        userID,
		PortfolioID:   portfolioID,
		TotalCash:     totalCash,
		AvailableCash: availableCash,
		BlockedCash:   blockedCash,
		Positions:     positions,
		UpdatedAt:     time.Now(),
	}

	m.mu.Lock()
	m.store[userID] = p
	m.mu.Unlock()
	return p, nil
}

func (m *Manager) ensureDefaultPortfolioRow(ctx context.Context, userID string) (int64, float64, float64, float64, error) {
	_, err := m.db.Exec(ctx, `
		INSERT INTO portfolios (user_id, name, total_cash, available_cash, blocked_cash, margin_locked)
		SELECT $1::uuid, 'Default', 100000.0, 100000.0, 0, 0
		WHERE NOT EXISTS (
			SELECT 1 FROM portfolios WHERE user_id = $1::uuid
		)
	`, userID)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	row := m.db.QueryRow(ctx,
		`SELECT id, total_cash, available_cash, blocked_cash FROM portfolios WHERE user_id = $1::uuid LIMIT 1`,
		userID,
	)
	var portfolioID int64
	var totalCash, availableCash, blockedCash float64
	if scanErr := row.Scan(&portfolioID, &totalCash, &availableCash, &blockedCash); scanErr != nil {
		return 0, 0, 0, 0, scanErr
	}

	return portfolioID, totalCash, availableCash, blockedCash, nil
}

func (m *Manager) userExists(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := m.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id::text = $1)`, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// flushWorker draws from the flush channel and writes snapshots to Postgres.
func (m *Manager) flushWorker() {
	batch := make(map[string]struct{})
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			m.flushAll()
			return
		case uid := <-m.flushCh:
			batch[uid] = struct{}{}
		case <-ticker.C:
			if len(batch) > 0 {
				for uid := range batch {
					if err := m.flushUser(uid); err != nil {
						log.Printf("[portfolio] flush error for %s: %v", uid, err)
					}
				}
				batch = make(map[string]struct{})
			}
		}
	}
}

func (m *Manager) flushUser(userID string) error {
	if m.db == nil {
		return nil
	}
	m.mu.RLock()
	p, ok := m.store[userID]
	if !ok {
		m.mu.RUnlock()
		return nil
	}

	// Take a snapshot under the read lock and release immediately
	posJSON := make([]map[string]interface{}, 0, len(p.Positions))
	for _, pos := range p.Positions {
		posJSON = append(posJSON, map[string]interface{}{
			"symbol":    pos.Symbol,
			"quantity":  pos.Quantity,
			"avg_entry": pos.AvgEntry,
		})
	}
	totalCash := p.TotalCash
	avail := p.AvailableCash
	blocked := p.BlockedCash
	portfolioID := p.PortfolioID
	m.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Update portfolio cash columns
	_, err = tx.Exec(ctx,
		`UPDATE portfolios SET total_cash=$1, available_cash=$2, blocked_cash=$3, updated_at=NOW()
		 WHERE id=$4`,
		totalCash, avail, blocked, portfolioID,
	)
	if err != nil {
		return fmt.Errorf("update portfolio cash: %w", err)
	}

	// Upsert positions
	for _, posMap := range posJSON {
		sym := posMap["symbol"].(string)
		qty := posMap["quantity"].(float64)
		avgEntry := posMap["avg_entry"].(float64)

		_, err = tx.Exec(ctx, `
			INSERT INTO positions (portfolio_id, instrument_id, net_quantity, average_entry_price, updated_at)
			SELECT $1, i.id, $3, $4, NOW()
			FROM instruments i WHERE i.symbol = $2
			ON CONFLICT (portfolio_id, instrument_id)
			DO UPDATE SET net_quantity=$3, average_entry_price=$4, updated_at=NOW()
		`, portfolioID, sym, qty, avgEntry)
		if err != nil {
			log.Printf("[portfolio] upsert position %s: %v", sym, err)
		}
	}

	return tx.Commit(ctx)
}

func (m *Manager) flushAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.store))
	for id := range m.store {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		if err := m.flushUser(id); err != nil {
			log.Printf("[portfolio] final flush error for %s: %v", id, err)
		}
	}
}

func (m *Manager) scheduleFLush(userID string) {
	select {
	case m.flushCh <- userID:
	default:
	}
}

// broadcastUpdate builds a snapshot and sends it to the user via WebSocket.
func (m *Manager) broadcastUpdate(userID string) {
	if m.wsHub == nil {
		return
	}

	m.mu.RLock()
	p, ok := m.store[userID]
	if !ok {
		m.mu.RUnlock()
		return
	}
	snapshot := buildSnapshot(p)
	m.mu.RUnlock()

	m.wsHub.BroadcastToUser(userID, &ws.Message{
		Type:      ws.EventPortfolio,
		Timestamp: time.Now().UnixMilli(),
		Data:      snapshot,
	})
}

// ─────────────────────────────────────────────
//  Snapshot builder (pure; caller holds lock)
// ─────────────────────────────────────────────

func buildSnapshot(p *Portfolio) *Snapshot {
	views := make([]PositionView, 0, len(p.Positions))
	var totalPnL, holdingsValue float64
	for _, pos := range p.Positions {
		if math_abs(pos.Quantity) < 1e-9 {
			continue
		}
		views = append(views, PositionView{
			Symbol:    pos.Symbol,
			Quantity:  pos.Quantity,
			AvgEntry:  pos.AvgEntry,
			MarkPrice: pos.MarkPrice,
			PnL:       pos.PnL,
		})
		totalPnL += pos.PnL
		holdingsValue += pos.MarkPrice * pos.Quantity
	}
	return &Snapshot{
		UserID:        p.UserID,
		TotalCash:     p.TotalCash,
		AvailableCash: p.AvailableCash,
		BlockedCash:   p.BlockedCash,
		Positions:     views,
		TotalPnL:      totalPnL,
		Equity:        p.AvailableCash + p.BlockedCash + holdingsValue,
		UpdatedAt:     p.UpdatedAt,
	}
}

// JSON helper (used for logging only – not exported)
func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func math_abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
