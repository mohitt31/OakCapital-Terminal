-- ============================================================
-- 2. instruments
-- ============================================================

CREATE TABLE instruments (
    id              BIGSERIAL    PRIMARY KEY,
    symbol          VARCHAR(20)  NOT NULL UNIQUE,   -- e.g. 'BTC-USD'
    base_currency   VARCHAR(10)  NOT NULL,
    quote_currency  VARCHAR(10)  NOT NULL,
    tick_size       NUMERIC(20, 8) NOT NULL,         -- Minimum price increment
    lot_size        NUMERIC(20, 8) NOT NULL,         -- Minimum quantity increment
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE
);
