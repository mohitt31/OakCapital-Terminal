-- ============================================================
-- 8. order_book_snapshots
-- ============================================================

CREATE TABLE order_book_snapshots (
    id              BIGSERIAL      PRIMARY KEY,
    instrument_id   BIGINT         NOT NULL REFERENCES instruments(id) ON DELETE CASCADE,
    bids            JSONB          NOT NULL DEFAULT '[]',     -- List of [price, quantity] pairs
    asks            JSONB          NOT NULL DEFAULT '[]',     -- List of [price, quantity] pairs
    mid_price       NUMERIC(20, 8),
    spread          NUMERIC(20, 8),
    snapped_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_book_snapshots_instrument ON order_book_snapshots(instrument_id, snapped_at);
