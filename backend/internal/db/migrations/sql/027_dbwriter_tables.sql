-- Migration: Create persistent storage tables for event bus consumers
-- Purpose: Store all durable events (trades, orders, errors, etc.) from Redis streams
-- Usage: Place in internal/db/migrations/sql/ directory

-- ============================================================
-- Order history table (append-only log)
-- ============================================================

CREATE TABLE IF NOT EXISTS order_history (
    id                BIGSERIAL PRIMARY KEY,
    order_id          BIGINT NOT NULL,
    user_id           TEXT,                                     -- FK to users table (optional)
    client_id         TEXT,                                     -- client-provided order ID for traceability
    symbol            VARCHAR(16) NOT NULL,
    side              VARCHAR(8) NOT NULL,                      -- 'buy' or 'sell'
    order_type        VARCHAR(16) NOT NULL,                     -- 'limit' or 'market'
    status            VARCHAR(32) NOT NULL,                     -- 'open', 'partial_filled', 'filled', 'cancelled'
    quantity          DECIMAL(20,8) NOT NULL,
    filled_qty        DECIMAL(20,8) NOT NULL DEFAULT 0,
    avg_price         DECIMAL(20,8) NOT NULL DEFAULT 0,
    updated_at        TIMESTAMPTZ NOT NULL,

    -- Every row is a snapshot at a point in time; (order_id, updated_at) uniquely identifies a state
    UNIQUE(order_id, updated_at)
);

CREATE INDEX IF NOT EXISTS idx_order_history_user ON order_history (user_id);
CREATE INDEX IF NOT EXISTS idx_order_history_symbol ON order_history (symbol);
CREATE INDEX IF NOT EXISTS idx_order_history_status ON order_history (status);

COMMENT ON TABLE order_history IS 'Append-only audit log of all order state transitions';
COMMENT ON COLUMN order_history.client_id IS 'User-supplied order ID for correlation with external systems';

-- ============================================================
-- System error log
-- ============================================================

CREATE TABLE IF NOT EXISTS system_event_log (
    id              BIGSERIAL PRIMARY KEY,
    event_type      VARCHAR(128) NOT NULL,                      -- error code or event class
    severity        VARCHAR(16) NOT NULL,                       -- 'debug', 'info', 'warn', 'error', 'critical'
    reference_id    BIGINT,                                     -- optional link to order_id, trade_id, etc.
    payload         JSONB,                                      -- contextual data
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_log_type ON system_event_log (event_type);
CREATE INDEX IF NOT EXISTS idx_event_log_severity ON system_event_log (severity);
CREATE INDEX IF NOT EXISTS idx_event_log_time ON system_event_log (created_at DESC);

COMMENT ON TABLE system_event_log IS 'Central logging table for system events, errors, alerts, health events';

-- ============================================================
-- User alerts (notifications)
-- ============================================================

CREATE TABLE IF NOT EXISTS user_alerts (
    id              BIGSERIAL PRIMARY KEY,
    user_id         TEXT NOT NULL,                              -- FK to users
    alert_type      VARCHAR(64) NOT NULL,                       -- 'price_threshold', 'portfolio_milestone', 'order_status', etc.
    symbol          VARCHAR(16),                                -- optional, if alert is symbol-specific
    message         TEXT NOT NULL,
    severity        VARCHAR(16) NOT NULL,                       -- 'info', 'warn', 'error'
    payload         JSONB,                                      -- additional context
    created_at      TIMESTAMPTZ NOT NULL,
    is_read         BOOLEAN DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_alerts_user ON user_alerts (user_id);
CREATE INDEX IF NOT EXISTS idx_alerts_unread ON user_alerts (user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_alerts_time ON user_alerts (created_at DESC);

COMMENT ON TABLE user_alerts IS 'User-facing notifications (portfolio changes, price alerts, order updates)';

-- ============================================================
-- Bot status log
-- ============================================================

CREATE TABLE IF NOT EXISTS bot_status_log (
    id              BIGSERIAL PRIMARY KEY,
    bot_id          TEXT NOT NULL,
    client_id       TEXT,                                       -- client managing this bot
    user_id         TEXT,                                       -- owner user (optional)
    status          VARCHAR(64) NOT NULL,                       -- 'running', 'paused', 'stopped', 'error'
    message         TEXT,
    payload         JSONB,                                      -- bot-specific metadata (position, PnL, etc.)
    created_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bot_log_bot ON bot_status_log (bot_id);
CREATE INDEX IF NOT EXISTS idx_bot_log_user ON bot_status_log (user_id);
CREATE INDEX IF NOT EXISTS idx_bot_log_time ON bot_status_log (created_at DESC);

COMMENT ON TABLE bot_status_log IS 'Audit trail of automated bot activity and state changes';

-- ============================================================
-- Portfolio snapshots (historical tracking)
-- ============================================================

CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             TEXT NOT NULL,                          -- FK to users
    total_cash          DECIMAL(20,8) NOT NULL,
    realized_pnl        DECIMAL(20,8),
    unrealized_pnl      DECIMAL(20,8),
    num_positions       INTEGER,                                -- count of open positions
    payload             JSONB,                                  -- full portfolio state
    snapshot_at         TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_portfolio_snap_user ON portfolio_snapshots (user_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_snap_time ON portfolio_snapshots (snapshot_at DESC);

COMMENT ON TABLE portfolio_snapshots IS 'Historical point-in-time portfolio state for audit and analysis';

-- ============================================================
-- Market data snapshots (optional, for historical analysis)
-- ============================================================

CREATE TABLE IF NOT EXISTS market_snapshots (
    id              BIGSERIAL PRIMARY KEY,
    instrument_id   BIGINT NOT NULL REFERENCES instruments(id),
    best_bid        DECIMAL(20,8),
    best_ask        DECIMAL(20,8),
    last_price      DECIMAL(20,8),
    day_volume      DECIMAL(20,8),
    snapshot_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_market_snap_instr ON market_snapshots (instrument_id);
CREATE INDEX IF NOT EXISTS idx_market_snap_time ON market_snapshots (snapshot_at DESC);

COMMENT ON TABLE market_snapshots IS 'Optional: periodic snapshots of market state for historical analysis';
