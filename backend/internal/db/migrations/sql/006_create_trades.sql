-- ============================================================
-- 6. trades
-- ============================================================

CREATE TABLE trades (
    id                         BIGSERIAL      PRIMARY KEY,
    instrument_id              BIGINT         NOT NULL REFERENCES instruments(id) ON DELETE CASCADE,
    buy_order_id               BIGINT         NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sell_order_id              BIGINT         NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    price                      NUMERIC(20, 8) NOT NULL,
    quantity                   NUMERIC(20, 8) NOT NULL,
    side_that_took_liquidity   VARCHAR(10)    NOT NULL CHECK (side_that_took_liquidity IN ('BUY', 'SELL')),
    executed_at                TIMESTAMPTZ(6) NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trades_instrument_id ON trades(instrument_id);
CREATE INDEX idx_trades_buy_order_id ON trades(buy_order_id);
CREATE INDEX idx_trades_sell_order_id ON trades(sell_order_id);
CREATE INDEX idx_trades_executed_at ON trades(executed_at);
