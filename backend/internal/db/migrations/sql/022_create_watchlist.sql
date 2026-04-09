-- Migration: user watchlist
-- Each row represents one symbol on one user's watchlist.
-- instrument_id is FK to instruments so we can JOIN for symbol/metadata.

CREATE TABLE IF NOT EXISTS watchlist_items (
    id             BIGSERIAL    PRIMARY KEY,
    user_id        UUID         NOT NULL REFERENCES users(id)        ON DELETE CASCADE,
    instrument_id  BIGINT       NOT NULL REFERENCES instruments(id)  ON DELETE CASCADE,
    added_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- A user can only add a given symbol once
    UNIQUE(user_id, instrument_id)
);

CREATE INDEX IF NOT EXISTS idx_watchlist_user     ON watchlist_items(user_id);
CREATE INDEX IF NOT EXISTS idx_watchlist_added_at ON watchlist_items(user_id, added_at DESC);
