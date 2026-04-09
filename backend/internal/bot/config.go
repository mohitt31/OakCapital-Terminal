package bot

import "os"

// BotConfig holds all tuneable trading parameters for a bot instance.
// Each bot gets its own config so multiple bots can run with different settings.
type BotConfig struct {
	ClientID string // unique identity for this bot instance

	// StrategyType can be "builtin" or "custom".
	StrategyType string
	// CustomScriptID is the ID of the user-uploaded script to run.
	CustomScriptID string
	// ScriptLanguage is the language of the custom script (e.g., "python").
	ScriptLanguage string
	// ScriptPath is the path to the custom script file.
	ScriptPath string

	// Quoting parameters (for built-in market maker)
	Spread                  float64
	BaseOrderSize           float64
	MaxPosition             float64
	InventorySkewFactor     float64
	InventoryRiskThreshold  float64
	DynamicSpreadMultiplier float64

	// CancelInterval controls how often stale orders are cancelled
	// (every N market_data ticks per symbol).
	CancelInterval int
}

// DefaultConfig returns a BotConfig with the production defaults
// (matching the original go_mmb constants).
func DefaultConfig() BotConfig {
	return BotConfig{
		ClientID:                "mmb",
		Spread:                  0.2,
		BaseOrderSize:           5,
		MaxPosition:             50,
		InventorySkewFactor:     0.01,
		InventoryRiskThreshold:  20,
		DynamicSpreadMultiplier: 0.1,
		CancelInterval:          5,
	}
}

// WsURL returns the websocket URL, checking env first.
func WsURL() string {
	if url := os.Getenv("WS_URL"); url != "" {
		return url
	}
	return "ws://localhost:8080/ws/internal"
}
