-- ============================================================
-- 3. portfolios
-- ============================================================

CREATE TABLE portfolios (
    id              BIGSERIAL      PRIMARY KEY,
    user_id         UUID           NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(100)   NOT NULL,
    total_cash      NUMERIC(20, 8) NOT NULL DEFAULT 0,
    available_cash  NUMERIC(20, 8) NOT NULL DEFAULT 0,
    blocked_cash    NUMERIC(20, 8) NOT NULL DEFAULT 0,   -- Locked by open orders
    margin_locked   NUMERIC(20, 8) NOT NULL DEFAULT 0,   -- Locked for short positions / margin
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_portfolios_user_id ON portfolios(user_id);
