package db

import (
	"context"
	"fmt"
	"time"
)

// WatchlistItem is one row returned from a user's watchlist query.
type WatchlistItem struct {
	Symbol   string    `json:"symbol"`
	AddedAt  time.Time `json:"added_at"`
}

// WatchlistRepo performs direct PostgreSQL reads/writes for user watchlists.
// Uses the global Pool — returns an error if the pool is nil (DB unavailable).
type WatchlistRepo struct {
	symbolMap map[string]int64 // symbol → instrument_id (built at startup)
}

// NewWatchlistRepo creates a WatchlistRepo backed by the given symbol→id map.
func NewWatchlistRepo(symbolMap map[string]int64) *WatchlistRepo {
	return &WatchlistRepo{symbolMap: symbolMap}
}

// Add inserts a symbol into the user's watchlist.
// Returns nil if the symbol is already on the watchlist (idempotent).
func (r *WatchlistRepo) Add(ctx context.Context, userID, symbol string) error {
	if Pool == nil {
		return fmt.Errorf("database unavailable")
	}
	instID, ok := r.symbolMap[symbol]
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}
	_, err := Pool.Exec(ctx, `
		INSERT INTO watchlist_items (user_id, instrument_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, instrument_id) DO NOTHING
	`, userID, instID)
	if err != nil {
		return fmt.Errorf("watchlist add: %w", err)
	}
	return nil
}

// Remove deletes a symbol from the user's watchlist.
// Returns nil if the symbol was not on the watchlist (idempotent).
func (r *WatchlistRepo) Remove(ctx context.Context, userID, symbol string) error {
	if Pool == nil {
		return fmt.Errorf("database unavailable")
	}
	instID, ok := r.symbolMap[symbol]
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}
	_, err := Pool.Exec(ctx, `
		DELETE FROM watchlist_items
		WHERE user_id = $1 AND instrument_id = $2
	`, userID, instID)
	if err != nil {
		return fmt.Errorf("watchlist remove: %w", err)
	}
	return nil
}

// List returns all symbols on the user's watchlist, ordered by when they were added.
func (r *WatchlistRepo) List(ctx context.Context, userID string) ([]WatchlistItem, error) {
	if Pool == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := Pool.Query(ctx, `
		SELECT i.symbol, w.added_at
		FROM watchlist_items w
		JOIN instruments i ON i.id = w.instrument_id
		WHERE w.user_id = $1
		ORDER BY w.added_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("watchlist list: %w", err)
	}
	defer rows.Close()

	var items []WatchlistItem
	for rows.Next() {
		var item WatchlistItem
		if err := rows.Scan(&item.Symbol, &item.AddedAt); err != nil {
			return nil, fmt.Errorf("watchlist scan: %w", err)
		}
		items = append(items, item)
	}
	if items == nil {
		items = []WatchlistItem{} // return empty slice, not nil
	}
	return items, nil
}

// Has checks whether a specific symbol is on the user's watchlist.
func (r *WatchlistRepo) Has(ctx context.Context, userID, symbol string) (bool, error) {
	if Pool == nil {
		return false, fmt.Errorf("database unavailable")
	}
	instID, ok := r.symbolMap[symbol]
	if !ok {
		return false, nil // unknown symbol is never on a watchlist
	}
	var exists bool
	err := Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM watchlist_items
			WHERE user_id = $1 AND instrument_id = $2
		)
	`, userID, instID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("watchlist has: %w", err)
	}
	return exists, nil
}
