package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LeaderboardEntry struct {
	UserID        string    `json:"user_id"`
	Username      string    `json:"username"`
	BotID         string    `json:"bot_id"`
	StrategyName  string    `json:"strategy_name"`
	Symbol        string    `json:"symbol"`
	Mode          string    `json:"mode"`
	PnL           float64   `json:"pnl"`
	Status        string    `json:"status"`
	UpdatedAt     time.Time `json:"updated_at"`
	IsPublic      bool      `json:"is_public"`
	ShareStrategy bool      `json:"share_strategy"`
	OwnedByMe     bool      `json:"owned_by_me"`
	RankingScope  string    `json:"ranking_scope"`
	RankingMetric string    `json:"ranking_metric"`
}

type LeaderboardPublishRequest struct {
	UserID        string
	BotID         string
	IsPublic      bool
	ShareStrategy bool
}

type LeaderboardRepo struct {
	pool *pgxpool.Pool
}

func NewLeaderboardRepo(pool *pgxpool.Pool) *LeaderboardRepo {
	if pool == nil {
		return nil
	}
	return &LeaderboardRepo{pool: pool}
}

func (r *LeaderboardRepo) ensureSchema(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS user_bot_leaderboard_settings (
			id BIGSERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			bot_id TEXT NOT NULL,
			is_public BOOLEAN NOT NULL DEFAULT FALSE,
			share_strategy BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (user_id, bot_id)
		);
		CREATE INDEX IF NOT EXISTS idx_user_bot_leaderboard_settings_user
			ON user_bot_leaderboard_settings(user_id);
		CREATE INDEX IF NOT EXISTS idx_user_bot_leaderboard_settings_public
			ON user_bot_leaderboard_settings(is_public, updated_at DESC);
		CREATE TABLE IF NOT EXISTS user_bot_pnl_history (
			id BIGSERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			bot_id TEXT NOT NULL,
			pnl DOUBLE PRECISION NOT NULL,
			captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			bucket_hour TIMESTAMPTZ NOT NULL,
			UNIQUE (user_id, bot_id, bucket_hour)
		);
		CREATE INDEX IF NOT EXISTS idx_user_bot_pnl_history_lookup
			ON user_bot_pnl_history(user_id, bot_id, captured_at DESC);
		CREATE INDEX IF NOT EXISTS idx_user_bot_pnl_history_recent
			ON user_bot_pnl_history(captured_at DESC);
	`)
	return err
}

func (r *LeaderboardRepo) UpsertPublishSettings(ctx context.Context, req LeaderboardPublishRequest) error {
	if r == nil || r.pool == nil {
		return nil
	}
	if err := r.ensureSchema(ctx); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_bot_leaderboard_settings (user_id, bot_id, is_public, share_strategy, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id, bot_id) DO UPDATE SET
			is_public = EXCLUDED.is_public,
			share_strategy = EXCLUDED.share_strategy,
			updated_at = NOW()
	`, req.UserID, req.BotID, req.IsPublic, req.ShareStrategy)
	return err
}

func (r *LeaderboardRepo) UserOwnsBot(ctx context.Context, userID, botID string) (bool, error) {
	if r == nil || r.pool == nil {
		return false, nil
	}
	if err := r.ensureSchema(ctx); err != nil {
		return false, err
	}
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_bot_pnl
			WHERE user_id = $1 AND bot_id = $2
		)
	`, userID, botID).Scan(&ok)
	return ok, err
}

func (r *LeaderboardRepo) List(ctx context.Context, userID string, includePublic bool, weeklyOnly bool) ([]LeaderboardEntry, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	if err := r.ensureSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		WITH latest AS (
			SELECT
				p.user_id,
				p.bot_id,
				p.strategy_name,
				p.symbol,
				p.mode,
				p.pnl,
				p.status,
				p.updated_at
			FROM user_bot_pnl p
		),
		baseline AS (
			SELECT
				l.user_id,
				l.bot_id,
				COALESCE(
					(
						SELECT h.pnl
						FROM user_bot_pnl_history h
						WHERE h.user_id::text = l.user_id
						  AND h.bot_id = l.bot_id
						  AND h.captured_at <= NOW() - INTERVAL '7 days'
						ORDER BY h.captured_at DESC
						LIMIT 1
					),
					(
						SELECT h.pnl
						FROM user_bot_pnl_history h
						WHERE h.user_id::text = l.user_id
						  AND h.bot_id = l.bot_id
						  AND h.captured_at >= NOW() - INTERVAL '7 days'
						ORDER BY h.captured_at ASC
						LIMIT 1
					),
					0
				) AS baseline_pnl,
				EXISTS (
					SELECT 1
					FROM user_bot_pnl_history h
					WHERE h.user_id::text = l.user_id
					  AND h.bot_id = l.bot_id
					  AND h.captured_at >= NOW() - INTERVAL '7 days'
				) AS has_recent_history
			FROM latest l
		)
		SELECT
			l.user_id,
			COALESCE(u.username, 'trader') AS username,
			l.bot_id,
			l.strategy_name,
			l.symbol,
			l.mode,
			CASE WHEN $3 = TRUE THEN (l.pnl - b.baseline_pnl) ELSE l.pnl END AS ranking_pnl,
			l.status,
			l.updated_at,
			COALESCE(s.is_public, false) AS is_public,
			COALESCE(s.share_strategy, false) AS share_strategy
		FROM latest l
		JOIN baseline b
			ON b.user_id = l.user_id AND b.bot_id = l.bot_id
		LEFT JOIN users u ON u.id::text = l.user_id
		LEFT JOIN user_bot_leaderboard_settings s
			ON s.user_id = l.user_id AND s.bot_id = l.bot_id
		WHERE
			l.user_id = $1
			OR (
				$2 = TRUE
				AND COALESCE(s.is_public, false) = TRUE
				AND l.user_id <> $1
			)
		AND ($3 = FALSE OR b.has_recent_history = TRUE OR l.user_id = $1)
		ORDER BY ranking_pnl DESC, l.updated_at DESC
		LIMIT 500
	`, userID, includePublic, weeklyOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LeaderboardEntry, 0, 64)
	for rows.Next() {
		var x LeaderboardEntry
		if err := rows.Scan(
			&x.UserID, &x.Username, &x.BotID, &x.StrategyName, &x.Symbol, &x.Mode,
			&x.PnL, &x.Status, &x.UpdatedAt, &x.IsPublic, &x.ShareStrategy,
		); err != nil {
			return nil, err
		}
		x.OwnedByMe = x.UserID == userID
		if weeklyOnly {
			x.RankingScope = "weekly"
			x.RankingMetric = "weekly_pnl"
		} else {
			x.RankingScope = "all"
			x.RankingMetric = "weekly_pnl"
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
