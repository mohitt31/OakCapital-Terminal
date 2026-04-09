-- ============================================================
-- 7. candles
-- ============================================================

CREATE TABLE candles (
    id               BIGSERIAL      PRIMARY KEY,
    instrument_id    BIGINT         NOT NULL REFERENCES instruments(id) ON DELETE CASCADE,
    interval_type    VARCHAR(10)    NOT NULL,                 -- 1m, 5m, 1h, 1d
    interval_seconds INT            NOT NULL,                 -- Numeric interval for custom range queries
    open_price       NUMERIC(20, 8) NOT NULL,
    high_price       NUMERIC(20, 8) NOT NULL,
    low_price        NUMERIC(20, 8) NOT NULL,
    close_price      NUMERIC(20, 8) NOT NULL,
    volume           NUMERIC(20, 8) NOT NULL DEFAULT 0,
    start_time       TIMESTAMPTZ      NOT NULL,
    end_time         TIMESTAMPTZ      NOT NULL,
    UNIQUE (instrument_id, interval_type, start_time)
);

CREATE INDEX idx_candles_instrument_interval ON candles(instrument_id, interval_type, start_time);
