-- ============================================================
-- 4. positions
-- ============================================================

CREATE TABLE positions (
    id                   BIGSERIAL      PRIMARY KEY,
    portfolio_id         BIGINT         NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    instrument_id        BIGINT         NOT NULL REFERENCES instruments(id) ON DELETE CASCADE,
    net_quantity         NUMERIC(20, 8) NOT NULL DEFAULT 0,   -- Positive = long, negative = short
    average_entry_price  NUMERIC(20, 8) NOT NULL DEFAULT 0,
    updated_at           TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    UNIQUE (portfolio_id, instrument_id)
);

CREATE INDEX idx_positions_portfolio_id ON positions(portfolio_id);
CREATE INDEX idx_positions_instrument_id ON positions(instrument_id);
