-- Per-user sim bot (BulBul) PnL snapshots for history and portfolio breakdown.
CREATE TABLE IF NOT EXISTS user_bot_pnl (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    bot_id TEXT NOT NULL,
    strategy_name TEXT NOT NULL DEFAULT '',
    symbol TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL DEFAULT 'simulation',
    pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'idle',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, bot_id)
);

CREATE INDEX IF NOT EXISTS idx_user_bot_pnl_user_updated ON user_bot_pnl (user_id, updated_at DESC);
