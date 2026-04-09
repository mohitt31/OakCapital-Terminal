CREATE TABLE IF NOT EXISTS user_bot_pnl_history (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
