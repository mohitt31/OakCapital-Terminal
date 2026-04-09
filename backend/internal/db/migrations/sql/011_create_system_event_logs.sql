-- ============================================================
-- 11. system_event_logs
-- ============================================================

CREATE TABLE system_event_logs (
    id             BIGSERIAL    PRIMARY KEY,
    event_type     VARCHAR(50)  NOT NULL,                    -- SYSTEM_START, ORDER_ERROR, etc.
    severity       VARCHAR(10)  NOT NULL DEFAULT 'INFO'
                   CHECK (severity IN ('INFO', 'WARN', 'ERROR')),
    reference_id   BIGINT,                                   -- Optional link to order/trade ID
    payload        JSONB        NOT NULL DEFAULT '{}',       -- Contextual data
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_system_event_logs_type ON system_event_logs(event_type);
CREATE INDEX idx_system_event_logs_severity ON system_event_logs(severity);
CREATE INDEX idx_system_event_logs_created_at ON system_event_logs(created_at);
