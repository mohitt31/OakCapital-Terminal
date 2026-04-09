-- ============================================================
-- 9. market_simulation_configs
-- ============================================================

CREATE TABLE market_simulation_configs (
    id               BIGSERIAL      PRIMARY KEY,
    instrument_id    BIGINT         NOT NULL REFERENCES instruments(id) ON DELETE CASCADE,
    simulation_type  VARCHAR(20)    NOT NULL CHECK (simulation_type IN ('GBM', 'SINE', 'REPLAY')),
    gbm_mu           NUMERIC(10, 5),
    gbm_sigma        NUMERIC(10, 5),
    gbm_seed         BIGINT,                                  -- For reproducible simulations
    last_price       NUMERIC(20, 8),                          -- For warm restarts
    is_active        BOOLEAN        NOT NULL DEFAULT FALSE,
    updated_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_market_sim_instrument ON market_simulation_configs(instrument_id);
