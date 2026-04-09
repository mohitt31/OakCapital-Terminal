package simbot

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// BotManager — manages the lifecycle of multiple simulation bot instances.
//
// Each bot runs in its own goroutine and is identified by a unique bot ID.
// The manager provides create/start/stop/list/status operations.
// ──────────────────────────────────────────────────────────────────────────────

type BotManager struct {
	mu       sync.RWMutex
	bots     map[string]*BotInstance
	updateCh chan BotStateUpdate // shared channel for all bot updates
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewBotManager creates a new manager.
func NewBotManager() *BotManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &BotManager{
		bots:     make(map[string]*BotInstance),
		updateCh: make(chan BotStateUpdate, 256),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// UpdateChan returns a read-only channel that receives state updates
// from all running bots. The caller can use this to fan-out to WebSocket clients.
func (m *BotManager) UpdateChan() <-chan BotStateUpdate {
	return m.updateCh
}

// StartBot creates and starts a new simulation bot with the given strategy graph.
func (m *BotManager) StartBot(botID string, symbol string, strategy StrategyGraph, mode BotMode, userID string, strategyLabel string, evalInterval time.Duration) error {
	m.mu.Lock()

	// Check if bot with this ID already exists and is running
	if existing, ok := m.bots[botID]; ok {
		if existing.Status == StatusRunning {
			m.mu.Unlock()
			return fmt.Errorf("bot %q is already running", botID)
		}
		// If stopped/errored, remove it so we can restart
		delete(m.bots, botID)
	}

	bot := NewBotInstance(botID, symbol, strategy, mode, userID, strategyLabel, evalInterval, m.updateCh)
	m.bots[botID] = bot
	m.mu.Unlock()

	// Start the bot (this launches a goroutine)
	if err := bot.Start(m.ctx); err != nil {
		m.mu.Lock()
		delete(m.bots, botID)
		m.mu.Unlock()
		return fmt.Errorf("start bot %q: %w", botID, err)
	}

	return nil
}

// StopBot stops a running bot by ID.
func (m *BotManager) StopBot(botID string) error {
	m.mu.RLock()
	bot, ok := m.bots[botID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("bot %q not found", botID)
	}

	bot.Stop()
	return nil
}

// GetStatus returns the current state of a bot.
func (m *BotManager) GetStatus(botID string) (*BotStateUpdate, error) {
	m.mu.RLock()
	bot, ok := m.bots[botID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("bot %q not found", botID)
	}

	state := bot.GetState()
	return &state, nil
}

// GetLogs returns the recent logs for a bot.
func (m *BotManager) GetLogs(botID string) ([]BotLog, error) {
	m.mu.RLock()
	bot, ok := m.bots[botID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("bot %q not found", botID)
	}

	bot.mu.RLock()
	defer bot.mu.RUnlock()

	logs := make([]BotLog, len(bot.Logs))
	copy(logs, bot.Logs)
	return logs, nil
}

// ListBots returns the state of all bots.
func (m *BotManager) ListBots() []BotStateUpdate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]BotStateUpdate, 0, len(m.bots))
	for _, bot := range m.bots {
		states = append(states, bot.GetState())
	}
	return states
}

// ListBotsForUser returns bots owned by the given user (empty userID matches none).
func (m *BotManager) ListBotsForUser(userID string) []BotStateUpdate {
	if userID == "" {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]BotStateUpdate, 0)
	for _, bot := range m.bots {
		if bot.UserID == userID {
			states = append(states, bot.GetState())
		}
	}
	return states
}

// StopBotForUser stops a bot only if it belongs to the user.
func (m *BotManager) StopBotForUser(botID, userID string) error {
	m.mu.RLock()
	bot, ok := m.bots[botID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("bot %q not found", botID)
	}
	if userID != "" && bot.UserID != "" && bot.UserID != userID {
		m.mu.RUnlock()
		return fmt.Errorf("forbidden")
	}
	m.mu.RUnlock()

	bot.Stop()
	return nil
}

// GetStatusForUser returns status and logs if the bot exists and belongs to the user.
func (m *BotManager) GetStatusForUser(botID, userID string) (BotStateUpdate, []BotLog, error) {
	m.mu.RLock()
	bot, ok := m.bots[botID]
	m.mu.RUnlock()
	if !ok {
		return BotStateUpdate{}, nil, fmt.Errorf("bot %q not found", botID)
	}
	if userID != "" && bot.UserID != "" && bot.UserID != userID {
		return BotStateUpdate{}, nil, fmt.Errorf("forbidden")
	}
	st := bot.GetState()
	logs, err := m.GetLogs(botID)
	return st, logs, err
}

// Close stops all bots and cleans up resources.
func (m *BotManager) Close() {
	m.cancel()

	m.mu.Lock()
	for id, bot := range m.bots {
		bot.Stop()
		delete(m.bots, id)
	}
	m.mu.Unlock()
}
