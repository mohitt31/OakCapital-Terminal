-- ============================================================
-- 5. orders
-- ============================================================

CREATE TABLE orders (
    id                  BIGSERIAL      PRIMARY KEY,
    client_order_id     VARCHAR(64)    NOT NULL,             -- Client-side traceability
    user_id             UUID           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    instrument_id       BIGINT         NOT NULL REFERENCES instruments(id) ON DELETE CASCADE,
    side                VARCHAR(10)    NOT NULL CHECK (side IN ('BUY', 'SELL')),
    order_type          VARCHAR(20)    NOT NULL CHECK (order_type IN ('LIMIT', 'MARKET')),
    price               NUMERIC(20, 8),                      -- NULL for MARKET orders
    quantity            NUMERIC(20, 8) NOT NULL,
    remaining_quantity  NUMERIC(20, 8) NOT NULL,
    status              VARCHAR(30)    NOT NULL DEFAULT 'OPEN'
                        CHECK (status IN ('OPEN', 'FILLED', 'CANCELLED', 'REJECTED')),
    fee                 NUMERIC(20, 8) NOT NULL DEFAULT 0,
    fee_asset           VARCHAR(20),
    is_synthetic        BOOLEAN        NOT NULL DEFAULT FALSE,  -- System-generated liquidity marker
    placed_at           TIMESTAMPTZ(6) NOT NULL DEFAULT NOW(),  -- Microsecond precision for price-time priority
    updated_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    cancelled_at        TIMESTAMPTZ,
    cancel_reason       VARCHAR(255)
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_instrument_id ON orders(instrument_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_placed_at ON orders(placed_at);
CREATE INDEX idx_orders_user_status ON orders(user_id, status);
