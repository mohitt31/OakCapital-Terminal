package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// safeNamePattern restricts strategy names to alphanumeric, underscores, and hyphens.
var safeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// BotInstance represents a single running bot.
type BotInstance struct {
	ID       string
	OwnerID  string // User who owns this bot
	Strategy Strategy
	Client   *ExchangeClient
	stopCh   chan struct{}
	running  bool
	config   BotConfig
}

// BotManager handles bot lifecycle: create, start, stop, list.
// It maintains a registry of strategy factories so new bot types
// can be registered at init-time.
type BotManager struct {
	mu                sync.Mutex
	bots              map[string]*BotInstance
	strategyFactories map[string]StrategyFactory
	broadcaster       Broadcaster
}

// Broadcaster defines the interface for broadcasting user-specific messages.
type Broadcaster interface {
	BroadcastToUser(userID string, msg interface{})
}

// NewManager creates a new BotManager.
func NewManager(broadcaster Broadcaster) *BotManager {
	return &BotManager{
		bots: make(map[string]*BotInstance),
		strategyFactories: map[string]StrategyFactory{
			"custom": NewCustomPythonStrategy,
		},
		broadcaster: broadcaster,
	}
}

// RegisterStrategy allows registering new strategy types at runtime.
func (m *BotManager) RegisterStrategy(name string, factory StrategyFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strategyFactories[name] = factory
}

// Create registers a new bot instance but does not start it.
func (m *BotManager) Create(ownerID, botID, strategyType string, cfg BotConfig) (*BotInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var strat Strategy
	var err error

	factory, ok := m.strategyFactories[strategyType]
	if !ok {
		return nil, fmt.Errorf("unknown strategy type: %q", strategyType)
	}
	strat, err = factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create strategy: %w", err)
	}

	inst := &BotInstance{
		ID:       botID,
		OwnerID:  ownerID,
		Strategy: strat,
		config:   cfg,
		Client:   NewExchangeClient(cfg.ClientID),
	}

	m.bots[inst.ID] = inst
	return inst, nil
}

// Start connects the bot to the exchange and spawns the event loop goroutine.
func (m *BotManager) Start(id string) error {
	m.mu.Lock()
	inst, ok := m.bots[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("bot %q not found", id)
	}
	if inst.running {
		m.mu.Unlock()
		return fmt.Errorf("bot %q already running", id)
	}
	inst.stopCh = make(chan struct{})
	inst.running = true
	m.mu.Unlock()

	wsURL := WsURL()
	if err := inst.Client.Connect(wsURL); err != nil {
		m.mu.Lock()
		if inst.running {
			inst.running = false
			select {
			case <-inst.stopCh:
			default:
				close(inst.stopCh)
			}
		}
		m.mu.Unlock()
		return fmt.Errorf("connect bot %q: %w", id, err)
	}

	go m.runLoop(inst)
	log.Printf("[manager] started bot %q (strategy=%s, ws=%s)", id, inst.Strategy.Name(), wsURL)
	return nil
}

// Stoppable is an optional interface that strategies can implement to clean up
// resources (e.g. Docker containers, gRPC connections) when the bot is stopped.
type Stoppable interface {
	Stop()
}

// Stop shuts down a running bot and cleans up its strategy resources.
func (m *BotManager) Stop(id string) error {
	m.mu.Lock()
	inst, ok := m.bots[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("bot %q not found", id)
	}
	running := inst.running
	if inst.running {
		inst.running = false
		select {
		case <-inst.stopCh:
		default:
			close(inst.stopCh)
		}
	}
	m.mu.Unlock()

	if running {
		inst.Client.Close()
		// Clean up strategy resources (Docker containers, gRPC connections, etc.)
		if stoppable, ok := inst.Strategy.(Stoppable); ok {
			stoppable.Stop()
		}
		log.Printf("[manager] stopped bot %q", id)
	}
	return nil
}

// Remove stops and removes a bot from the manager.
func (m *BotManager) Remove(id string) {
	m.mu.Lock()
	delete(m.bots, id)
	m.mu.Unlock()
}

// Get returns a bot instance by ID.
func (m *BotManager) Get(id string) (BotInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.bots[id]
	if !ok {
		return BotInfo{}, fmt.Errorf("bot %q not found", id)
	}
	return BotInfo{
		ID:       inst.ID,
		OwnerID:  inst.OwnerID,
		Strategy: inst.Strategy.Name(),
		Running:  inst.running,
	}, nil
}

// List returns the IDs and strategy names of all bots.
func (m *BotManager) List() []BotInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]BotInfo, 0, len(m.bots))
	for _, inst := range m.bots {
		result = append(result, BotInfo{
			ID:       inst.ID,
			OwnerID:  inst.OwnerID,
			Strategy: inst.Strategy.Name(),
			Running:  inst.running,
		})
	}
	return result
}

// BotInfo is a summary of a bot instance for listing.
type BotInfo struct {
	ID       string `json:"id"`
	OwnerID  string `json:"owner_id"`
	Strategy string `json:"strategy"`
	Running  bool   `json:"running"`
}

// runLoop is the main event loop for a bot instance.
// It reads messages from the exchange and dispatches to the strategy.
func (m *BotManager) runLoop(inst *BotInstance) {
	defer func() {
		m.mu.Lock()
		inst.running = false
		m.mu.Unlock()
		log.Printf("[manager] bot %q event loop exited", inst.ID)
	}()

	// Broadcast status every 5 seconds
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	for {
		select {
		case <-inst.stopCh:
			return
		case <-statusTicker.C:
			m.broadcastStatus(inst)
		default:
		}

		msg, err := inst.Client.Recv()
		if err != nil {
			select {
			case <-inst.stopCh:
				return // expected close
			default:
				log.Printf("[manager] bot %q recv error: %v", inst.ID, err)
				return
			}
		}

		switch msg.Type {
		case "market_data":
			orders := inst.Strategy.OnMarketData(msg)
			for _, order := range orders {
				if err := inst.Client.Send(order); err != nil {
					log.Printf("[manager] bot %q send error: %v", inst.ID, err)
				}
			}
		case "fill":
			inst.Strategy.OnFill(msg)
		case "ack":
			inst.Strategy.OnAck(msg)
		case "trade":
			log.Printf("[%s] trade %s price=%.2f qty=%.0f side=%s",
				inst.ID, msg.Symbol, msg.Price, msg.Quantity, msg.Side)
		}
	}
}

func (m *BotManager) broadcastStatus(inst *BotInstance) {
	if m.broadcaster == nil || inst.OwnerID == "" {
		return
	}

	pnl, err := inst.Strategy.GetPNL()
	if err != nil {
		pnl = 0 // or some error indicator
	}

	statusMsg := "running"
	if !inst.running {
		statusMsg = "stopped"
	}

	m.broadcaster.BroadcastToUser(inst.OwnerID, map[string]interface{}{
		"type":      "bot_status",
		"timestamp": time.Now().UnixMilli(),
		"data": map[string]interface{}{
			"bot_id":    inst.ID,
			"status":    statusMsg,
			"pnl":       pnl,
			"timestamp": time.Now().UnixMilli(),
		},
	})
}

// SaveUserStrategy saves a user's custom strategy.
func (m *BotManager) SaveUserStrategy(userID, strategyName string, strategyData []byte) error {
	dir := "strategies"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create strategies directory: %w", err)
	}

	// Sanitize strategyName: replace spaces, lowercase, then validate against
	// a strict allowlist to prevent path traversal (e.g. "../../etc/passwd").
	safeName := strings.ReplaceAll(strategyName, " ", "_")
	safeName = filepath.Base(safeName) // strip any directory components

	if !safeNamePattern.MatchString(safeName) || safeName == "." || safeName == ".." {
		return fmt.Errorf("invalid strategy name: must contain only alphanumeric characters, underscores, and hyphens")
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s_%s.json", userID, safeName))

	var normalized json.RawMessage
	if !json.Valid(strategyData) {
		return fmt.Errorf("invalid strategy JSON")
	}
	normalized = strategyData

	payload, err := json.Marshal(map[string]json.RawMessage{
		"name":     json.RawMessage(fmt.Sprintf("%q", strategyName)),
		"strategy": normalized,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal strategy payload: %w", err)
	}

	if err := os.WriteFile(filename, payload, 0644); err != nil {
		return fmt.Errorf("failed to write strategy file: %w", err)
	}

	log.Printf("[manager] saved strategy for user %s: %s", userID, filename)
	return nil
}

type SavedStrategy struct {
	Name      string
	UpdatedAt time.Time
}

func (m *BotManager) ListUserStrategies(userID string) ([]SavedStrategy, error) {
	pattern := filepath.Join("strategies", fmt.Sprintf("%s_*.json", userID))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	out := make([]SavedStrategy, 0, len(files))
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		name := inferStrategyDisplayName(userID, file)
		if parsed := parseStoredStrategy(content); parsed.Name != "" {
			name = parsed.Name
		}
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		out = append(out, SavedStrategy{Name: name, UpdatedAt: info.ModTime().UTC()})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (m *BotManager) LoadUserStrategy(userID, strategyName string) ([]byte, error) {
	safeName := strings.ReplaceAll(strategyName, " ", "_")
	safeName = filepath.Base(safeName)
	if !safeNamePattern.MatchString(safeName) || safeName == "." || safeName == ".." {
		return nil, fmt.Errorf("invalid strategy name")
	}

	filename := filepath.Join("strategies", fmt.Sprintf("%s_%s.json", userID, safeName))
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read strategy file: %w", err)
	}
	parsed := parseStoredStrategy(content)
	if len(parsed.Strategy) > 0 {
		return parsed.Strategy, nil
	}
	return content, nil
}

type storedStrategy struct {
	Name     string          `json:"name"`
	Strategy json.RawMessage `json:"strategy"`
}

func parseStoredStrategy(content []byte) storedStrategy {
	var parsed storedStrategy
	_ = json.Unmarshal(content, &parsed)
	return parsed
}

func inferStrategyDisplayName(userID, path string) string {
	base := filepath.Base(path)
	prefix := userID + "_"
	name := strings.TrimSuffix(base, ".json")
	name = strings.TrimPrefix(name, prefix)
	return strings.ReplaceAll(name, "_", " ")
}
