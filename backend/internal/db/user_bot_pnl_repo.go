package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UserBotPnLRow is a persisted snapshot of one bot's metrics for a user.
type UserBotPnLRow struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	BotID        string    `json:"bot_id"`
	StrategyName string    `json:"strategy_name"`
	Symbol       string    `json:"symbol"`
	Mode         string    `json:"mode"`
	PnL          float64   `json:"pnl"`
	Status       string    `json:"status"`
	UpdatedAt    time.Time `json:"updated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserBotPnLRepo persists BulBul / simbot PnL per user for portfolio breakdown.
type UserBotPnLRepo struct {
	pool *pgxpool.Pool
}

func NewUserBotPnLRepo(pool *pgxpool.Pool) *UserBotPnLRepo {
	if pool == nil {
		return nil
	}
	return &UserBotPnLRepo{pool: pool}
}

// Upsert stores or updates the latest PnL for a user's bot instance.
func (r *UserBotPnLRepo) Upsert(ctx context.Context, row UserBotPnLRow) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_bot_pnl (user_id, bot_id, strategy_name, symbol, mode, pnl, status, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (user_id, bot_id) DO UPDATE SET
			strategy_name = EXCLUDED.strategy_name,
			symbol = EXCLUDED.symbol,
			mode = EXCLUDED.mode,
			pnl = EXCLUDED.pnl,
			status = EXCLUDED.status,
			updated_at = NOW()
	`, row.UserID, row.BotID, row.StrategyName, row.Symbol, row.Mode, row.PnL, row.Status)
	if err != nil {
		return err
	}
	// Persist a time-series sample for leaderboard weekly delta calculations.
	_, err = r.pool.Exec(ctx, `
		INSERT INTO user_bot_pnl_history (user_id, bot_id, pnl, captured_at, bucket_hour)
		VALUES ($1, $2, $3, NOW(), date_trunc('hour', NOW()))
		ON CONFLICT (user_id, bot_id, bucket_hour) DO UPDATE SET
			pnl = EXCLUDED.pnl,
			captured_at = NOW()
	`, row.UserID, row.BotID, row.PnL)
	return err
}

// ListByUser returns all recorded bot rows for portfolio distribution (most recent first).
func (r *UserBotPnLRepo) ListByUser(ctx context.Context, userID string) ([]UserBotPnLRow, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, bot_id, strategy_name, symbol, mode, pnl, status, updated_at, created_at
		FROM user_bot_pnl
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT 200
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserBotPnLRow
	for rows.Next() {
		var x UserBotPnLRow
		if err := rows.Scan(&x.ID, &x.UserID, &x.BotID, &x.StrategyName, &x.Symbol, &x.Mode, &x.PnL, &x.Status, &x.UpdatedAt, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
