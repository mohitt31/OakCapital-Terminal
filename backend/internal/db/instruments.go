package db

import (
	"context"
	"fmt"
)

// InstrumentSeed is the minimal data needed to upsert an instrument row.
type InstrumentSeed struct {
	Symbol        string
	BaseCurrency  string
	QuoteCurrency string
	TickSize      float64
	LotSize       float64
}

// SeedInstruments upserts the given instruments into the database and returns
// a map of symbol → instrument_id for use by downstream writers.
func SeedInstruments(ctx context.Context, seeds []InstrumentSeed) (map[string]int64, error) {
	if Pool == nil {
		return nil, fmt.Errorf("database pool not initialized")
	}

	m := make(map[string]int64, len(seeds))

	for _, s := range seeds {
		var id int64
		err := Pool.QueryRow(ctx, `
			INSERT INTO instruments (symbol, base_currency, quote_currency, tick_size, lot_size)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (symbol) DO UPDATE SET is_active = TRUE
			RETURNING id
		`, s.Symbol, s.BaseCurrency, s.QuoteCurrency, s.TickSize, s.LotSize).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("seed instrument %s: %w", s.Symbol, err)
		}
		m[s.Symbol] = id
	}

	fmt.Printf("Seeded %d instruments\n", len(m))
	return m, nil
}
