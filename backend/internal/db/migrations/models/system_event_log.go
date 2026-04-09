package models

import (
	"encoding/json"
	"time"
)

// SystemEventLog records system events for debugging and alerting.
type SystemEventLog struct {
	ID          int64           `json:"id" db:"id"`
	EventType   string          `json:"event_type" db:"event_type"`               // SYSTEM_START, ORDER_ERROR, etc.
	Severity    Severity        `json:"severity" db:"severity"`                   // INFO, WARN, ERROR
	ReferenceID *int64          `json:"reference_id,omitempty" db:"reference_id"` // Optional link to order/trade
	Payload     json.RawMessage `json:"payload" db:"payload"`                     // JSONB contextual data
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}
