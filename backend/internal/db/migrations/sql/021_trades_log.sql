-- Migration: trades_log table for event-driven trade persistence
-- Separate from the trades table (which has FK to orders) because engine order IDs
-- are internal sequential integers, not DB-assigned row IDs.

CREATE TABLE IF NOT EXISTS trades_log (
    id                       BIGSERIAL PRIMARY KEY,
    instrument_id            BIGINT NOT NULL REFERENCES instruments(id),
    engine_buy_order_id      BIGINT NOT NULL,
    engine_sell_order_id     BIGINT NOT NULL,
    price                    DECIMAL(20,8) NOT NULL,
    quantity                 DECIMAL(20,8) NOT NULL,
    taker_side               VARCHAR(16) NOT NULL,
    executed_at              TIMESTAMPTZ NOT NULL,
    UNIQUE(instrument_id, engine_buy_order_id, engine_sell_order_id, executed_at)
);

CREATE INDEX IF NOT EXISTS idx_trades_log_instrument_time ON trades_log(instrument_id, executed_at DESC);
CREATE INDEX IF NOT EXISTS idx_trades_log_buy_order ON trades_log(engine_buy_order_id);
CREATE INDEX IF NOT EXISTS idx_trades_log_sell_order ON trades_log(engine_sell_order_id);
