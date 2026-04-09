// Package syslog provides a structured, buffered system event logger that
// writes to both stdout (immediately) and PostgreSQL (in batches every 5 s).
//
// It is intentionally decoupled from any specific ORM: callers inject a
// FlushFunc that receives a slice of Events and performs the actual INSERT.
// Pass nil for FlushFunc to get stdout-only logging (useful in tests or when
// the DB is not yet available during startup).
//
// PostgreSQL schema (add to migrations):
//
//	CREATE TABLE system_events (
//	    id           BIGSERIAL   PRIMARY KEY,
//	    service_name VARCHAR(64) NOT NULL,
//	    event_type   VARCHAR(32) NOT NULL,
//	    severity     VARCHAR(16) NOT NULL,
//	    message      TEXT,
//	    details      JSONB,
//	    duration_ms  INT,
//	    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
//	);
//	CREATE INDEX idx_sysevt_service_time ON system_events (service_name, created_at DESC);
//	CREATE INDEX idx_sysevt_severity_time ON system_events (severity,     created_at DESC);
//
// Usage:
//
//	logger := syslog.New("candle-builder", func(evs []syslog.Event) error {
//	    // INSERT batch into PostgreSQL here
//	    return nil
//	})
//	defer logger.Close()
//	logger.Log(syslog.EventStart, syslog.SevInfo, "started", nil)
package syslog

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// EventType classifies what kind of system event occurred.
type EventType string

const (
	EventStart       EventType = "START"
	EventShutdown    EventType = "SHUTDOWN"
	EventError       EventType = "ERROR"
	EventHealthOK    EventType = "HEALTH_CHECK_OK"
	EventHealthFail  EventType = "HEALTH_CHECK_FAIL"
	EventReconnect   EventType = "RECONNECT"
	EventMetrics     EventType = "METRICS"
	EventBatchWrite  EventType = "BATCH_WRITE"
	EventConsumerLag EventType = "CONSUMER_LAG"
)

// Severity reflects the urgency of a system event.
type Severity string

const (
	SevInfo  Severity = "INFO"
	SevWarn  Severity = "WARN"
	SevError Severity = "ERROR"
	SevFatal Severity = "FATAL"
)

// ---------------------------------------------------------------------------
// Event
// ---------------------------------------------------------------------------

// Event is a single structured system log entry.
type Event struct {
	ServiceName string            `json:"service_name"`
	EventType   EventType         `json:"event_type"`
	Severity    Severity          `json:"severity"`
	Message     string            `json:"message"`
	Details     map[string]any    `json:"details,omitempty"`
	DurationMs  int               `json:"duration_ms,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Logger
// ---------------------------------------------------------------------------

// FlushFunc is called periodically with a batch of buffered events.
// Return nil to confirm the write; return a non-nil error to retry on the
// next tick (the buffer is preserved on failure).
type FlushFunc func([]Event) error

// Logger buffers system events and batch-writes them via FlushFunc.
// It is safe for concurrent use.
type Logger struct {
	serviceName string
	flush       FlushFunc

	mu          sync.Mutex
	buffer      []Event
	flushTicker *time.Ticker
	done        chan struct{}
	wg          sync.WaitGroup
}

// New creates a Logger for the named service.
// flushFn may be nil — in that case events are only written to stdout.
// The flush interval is fixed at 5 seconds which keeps DB write load low
// while still providing near-real-time visibility.
func New(serviceName string, flushFn FlushFunc) *Logger {
	l := &Logger{
		serviceName: serviceName,
		flush:       flushFn,
		flushTicker: time.NewTicker(5 * time.Second),
		done:        make(chan struct{}),
	}
	l.wg.Add(1)
	go l.flushLoop()
	return l
}

// Log records a system event.  The event is printed to stdout immediately
// and queued for the next batch DB write.
func (l *Logger) Log(eType EventType, sev Severity, msg string, details map[string]any) {
	ev := l.newEvent(eType, sev, msg, details, 0)
	l.printEvent(ev)
	l.enqueue(ev)
}

// LogWithDuration is like Log but also records how long an operation took.
func (l *Logger) LogWithDuration(eType EventType, sev Severity, msg string, duration time.Duration, details map[string]any) {
	ev := l.newEvent(eType, sev, msg, details, int(duration.Milliseconds()))
	l.printEvent(ev)
	l.enqueue(ev)
}

// Close flushes any remaining buffered events and stops the background goroutine.
// Always defer Close() in main after creating a Logger.
func (l *Logger) Close() {
	close(l.done)
	l.wg.Wait()
	l.flushTicker.Stop()
	l.doFlush() // drain any remaining events
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (l *Logger) newEvent(eType EventType, sev Severity, msg string, details map[string]any, durationMs int) Event {
	return Event{
		ServiceName: l.serviceName,
		EventType:   eType,
		Severity:    sev,
		Message:     msg,
		Details:     details,
		DurationMs:  durationMs,
		Timestamp:   time.Now(),
	}
}

func (l *Logger) printEvent(ev Event) {
	if ev.DurationMs > 0 {
		log.Printf("[%s] %s | %s | %s (%dms)", ev.ServiceName, ev.Severity, ev.EventType, ev.Message, ev.DurationMs)
	} else {
		log.Printf("[%s] %s | %s | %s", ev.ServiceName, ev.Severity, ev.EventType, ev.Message)
	}
}

func (l *Logger) enqueue(ev Event) {
	l.mu.Lock()
	l.buffer = append(l.buffer, ev)
	l.mu.Unlock()
}

func (l *Logger) flushLoop() {
	defer l.wg.Done()
	for {
		select {
		case <-l.done:
			return
		case <-l.flushTicker.C:
			l.doFlush()
		}
	}
}

func (l *Logger) doFlush() {
	if l.flush == nil {
		return
	}

	l.mu.Lock()
	if len(l.buffer) == 0 {
		l.mu.Unlock()
		return
	}
	batch := l.buffer
	l.buffer = nil
	l.mu.Unlock()

	if err := l.flush(batch); err != nil {
		// Put events back so they are retried on the next tick.
		log.Printf("[syslog] flush failed for %s (%d events): %v — will retry", l.serviceName, len(batch), err)
		l.mu.Lock()
		l.buffer = append(batch, l.buffer...)
		l.mu.Unlock()
	}
}

// ---------------------------------------------------------------------------
// Convenience helpers used by services
// ---------------------------------------------------------------------------

// StartupLog emits a standard START event with the given metadata.
func (l *Logger) StartupLog(details map[string]any) {
	l.Log(EventStart, SevInfo, fmt.Sprintf("%s starting", l.serviceName), details)
}

// ShutdownLog emits a standard SHUTDOWN event.
func (l *Logger) ShutdownLog() {
	l.Log(EventShutdown, SevInfo, fmt.Sprintf("%s shutting down", l.serviceName), nil)
}

// ErrorLog emits an ERROR event with an optional error value.
func (l *Logger) ErrorLog(msg string, err error, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	if err != nil {
		details["error"] = err.Error()
	}
	l.Log(EventError, SevError, msg, details)
}
