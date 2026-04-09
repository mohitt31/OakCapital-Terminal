-- ============================================================
-- 10. bot_configs
-- ============================================================

CREATE TABLE bot_configs (
    id              BIGSERIAL    PRIMARY KEY,
    user_id         UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    instrument_id   BIGINT       NOT NULL REFERENCES instruments(id) ON DELETE CASCADE,
    bot_type        VARCHAR(30)  NOT NULL,                   -- e.g. MARKET_MAKER
    status          VARCHAR(20)  NOT NULL DEFAULT 'STOPPED'
                    CHECK (status IN ('RUNNING', 'STOPPED', 'PAUSED')),
    strategy_params JSONB        NOT NULL DEFAULT '{}',
    risk_limit      JSONB        NOT NULL DEFAULT '{}',
    last_run_at     TIMESTAMPTZ,
    error_log       TEXT,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bot_configs_user_id ON bot_configs(user_id);
CREATE INDEX idx_bot_configs_instrument_id ON bot_configs(instrument_id);
