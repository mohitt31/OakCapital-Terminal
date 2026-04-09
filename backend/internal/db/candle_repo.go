package db

import (
	"context"
	"fmt"
	"time"

	"synthbull/internal/eventbus"
)

// CandleRepo reads and writes candle rows, translating between the eventbus
// wire format (symbol string, cents-based int64 prices) and the normalised
// PostgreSQL schema (instrument_id, NUMERIC prices).
type CandleRepo struct {
	symbolMap map[string]int64 // symbol → instrument_id
}

// NewCandleRepo creates a CandleRepo backed by the given symbol→id mapping.
func NewCandleRepo(symbolMap map[string]int64) *CandleRepo {
	return &CandleRepo{symbolMap: symbolMap}
}

// CandleRow is the shape returned by QueryCandles — cents-based so the REST
// handler can forward it to the frontend without conversion.
type CandleRow struct {
	Time   int64 `json:"time"`
	Open   int64 `json:"open"`
	High   int64 `json:"high"`
	Low    int64 `json:"low"`
	Close  int64 `json:"close"`
	Volume int64 `json:"volume"`
}

func intervalSeconds(interval string) int {
	switch interval {
	case "1s":
		return 1
	case "5s":
		return 5
	case "1m":
		return 60
	case "5m":
		return 300
	case "1h":
		return 3600
	case "1d":
		return 86400
	default:
		return 1
	}
}

// InsertBatch upserts a slice of closed CandleEvents into the candles table.
func (r *CandleRepo) InsertBatch(ctx context.Context, batch []eventbus.CandleEvent) error {
	if Pool == nil {
		return fmt.Errorf("database pool not initialized")
	}
	if len(batch) == 0 {
		return nil
	}

	tx, err := Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, c := range batch {
		instID, ok := r.symbolMap[c.Symbol]
		if !ok {
			continue
		}
		secs := intervalSeconds(c.Interval)
		endTime := c.Timestamp.Add(time.Duration(secs) * time.Second)

		_, err := tx.Exec(ctx, `
			INSERT INTO candles
				(instrument_id, interval_type, interval_seconds,
				 open_price, high_price, low_price, close_price, volume,
				 start_time, end_time)
			VALUES ($1,$2,$3, $4,$5,$6,$7,$8, $9,$10)
			ON CONFLICT (instrument_id, interval_type, start_time)
			DO UPDATE SET
				high_price  = GREATEST(candles.high_price, EXCLUDED.high_price),
				low_price   = LEAST(candles.low_price, EXCLUDED.low_price),
				close_price = EXCLUDED.close_price,
				volume      = EXCLUDED.volume
		`,
			instID, c.Interval, secs,
			float64(c.Open)/100, float64(c.High)/100, float64(c.Low)/100, float64(c.Close)/100,
			c.Volume,
			c.Timestamp, endTime,
		)
		if err != nil {
			return fmt.Errorf("insert candle %s/%s: %w", c.Symbol, c.Interval, err)
		}
	}

	return tx.Commit(ctx)
}

// QueryCandles returns historical candles for a symbol+interval, most recent
// last (ascending start_time). Prices are returned in cents for direct
// consumption by the frontend.
//
// When before > 0 the query only returns candles with start_time strictly
// before that unix timestamp, enabling cursor-based backward pagination.
func (r *CandleRepo) QueryCandles(ctx context.Context, symbol, interval string, limit int, before int64) ([]CandleRow, error) {
	if Pool == nil {
		return nil, fmt.Errorf("database pool not initialized")
	}

	instID, ok := r.symbolMap[symbol]
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	query := `SELECT start_time, open_price, high_price, low_price, close_price, volume FROM candles WHERE instrument_id = $1 AND interval_type = $2`
	args := []interface{}{instID, interval}

	if before > 0 {
		query += ` AND start_time < $3 ORDER BY start_time DESC LIMIT $4`
		args = append(args, time.Unix(before, 0), limit)
	} else {
		query += ` ORDER BY start_time DESC LIMIT $3`
		args = append(args, limit)
	}

	rows, err := Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query candles: %w", err)
	}
	defer rows.Close()

	var result []CandleRow
	for rows.Next() {
		var startTime time.Time
		var o, h, l, c, vol float64
		if err := rows.Scan(&startTime, &o, &h, &l, &c, &vol); err != nil {
			return nil, fmt.Errorf("scan candle: %w", err)
		}
		result = append(result, CandleRow{
			Time:   startTime.Unix(),
			Open:   int64(o * 100),
			High:   int64(h * 100),
			Low:    int64(l * 100),
			Close:  int64(c * 100),
			Volume: int64(vol),
		})
	}

	// Reverse so the array is in ascending time order.
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}
