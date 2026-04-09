// Market manager — manages multiple symbols; each gets its own C++ matching engine,
// GBM generator, and mutex. DefaultSymbols registers international equities + crypto.
package market

import (
	"fmt"
	"log"
	"sync"

	"synthbull/internal/engine"
)

// AssetClass groups symbols with similar trading characteristics.
type AssetClass string

const (
	Crypto AssetClass = "crypto"
	Stock  AssetClass = "stock"
	ETF    AssetClass = "etf"
)

// SymbolInfo holds everything for one tradeable symbol.
type SymbolInfo struct {
	Symbol    string
	Class     AssetClass
	Book      *engine.Handle
	Mu        sync.Mutex
	Generator *Generator

	// Optional trade callback
	onTrade func(symbol string, side int, result engine.OrderResult)
}

// SetOnTrade sets a callback that fires when trades occur for this symbol.
func (si *SymbolInfo) SetOnTrade(callback func(symbol string, side int, result engine.OrderResult)) {
	si.onTrade = callback
	si.Generator.SetOnTrade(func(side int, result engine.OrderResult) {
		if si.onTrade != nil {
			si.onTrade(si.Symbol, side, result)
		}
	})
}

// Manager holds all symbols and their engines.
type Manager struct {
	mu      sync.RWMutex
	symbols map[string]*SymbolInfo
}

// NewManager creates an empty market manager.
func NewManager() *Manager {
	return &Manager{
		symbols: make(map[string]*SymbolInfo),
	}
}

// AddSymbol registers a new tradeable symbol with its own engine and generator.
// Each symbol gets a unique order ID range to avoid collisions.
func (m *Manager) AddSymbol(symbol string, class AssetClass, cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.symbols[symbol]; exists {
		return fmt.Errorf("symbol %q already exists", symbol)
	}

	book := engine.New()
	info := &SymbolInfo{
		Symbol: symbol,
		Class:  class,
		Book:   book,
	}
	info.Generator = New(cfg, book, &info.Mu)
	info.Generator.SetSymbol(symbol)

	// Give each symbol a unique order ID range (10M apart) so IDs never collide
	info.Generator.SetStartOrderID(int64(len(m.symbols)+1) * 10_000_000)

	m.symbols[symbol] = info
	log.Printf("[market] registered symbol %s (class=%s, price=%.2f)", symbol, class, cfg.InitialPrice)
	return nil
}

// StartAll starts generators for every registered symbol.
func (m *Manager) StartAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, info := range m.symbols {
		info.Generator.Start()
	}
	log.Printf("[market] started all %d symbols", len(m.symbols))
}

// StopAll stops all generators. The engines remain open so you can still
// read book state (depth, trades). Call CloseAll when fully done.
func (m *Manager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, info := range m.symbols {
		info.Generator.Stop()
	}
	log.Printf("[market] stopped all generators")
}

// CloseAll closes all engine handles and frees C++ memory.
// Call this on server shutdown after StopAll.
func (m *Manager) CloseAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, info := range m.symbols {
		info.Book.Close()
	}
	log.Printf("[market] closed all engines")
}

// GetSymbol returns the SymbolInfo for a given symbol, or nil if not found.
func (m *Manager) GetSymbol(symbol string) *SymbolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.symbols[symbol]
}

// ListSymbols returns all registered symbol names.
func (m *Manager) ListSymbols() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.symbols))
	for s := range m.symbols {
		result = append(result, s)
	}
	return result
}

// ListByClass returns symbol names for a given asset class.
func (m *Manager) ListByClass(class AssetClass) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []string
	for s, info := range m.symbols {
		if info.Class == class {
			result = append(result, s)
		}
	}
	return result
}

// SymbolCount returns total number of registered symbols.
func (m *Manager) SymbolCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.symbols)
}

// AddSymbolWithHandle registers a symbol whose matching engine is supplied by the
// caller (e.g. the REST API's BookManager). The GBM generator submits orders into
// this external handle so that simulated and user-submitted orders share one book.
// bookMu must be the same mutex that guards all operations on the handle.
func (m *Manager) AddSymbolWithHandle(symbol string, class AssetClass, cfg Config, book *engine.Handle, bookMu *sync.Mutex) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.symbols[symbol]; exists {
		return fmt.Errorf("symbol %q already exists", symbol)
	}

	info := &SymbolInfo{
		Symbol: symbol,
		Class:  class,
		Book:   book,
	}
	info.Generator = New(cfg, book, bookMu)
	info.Generator.SetSymbol(symbol)
	info.Generator.SetStartOrderID(int64(len(m.symbols)+1) * 10_000_000)

	m.symbols[symbol] = info
	log.Printf("[market] registered symbol %s (class=%s, price=%.2f, shared-book)", symbol, class, cfg.InitialPrice)
	return nil
}

// DefaultSymbols registers GBM-driven books for international equities + crypto.
func (m *Manager) DefaultSymbols() {
	for _, preset := range IndianStockPresets() {
		_ = m.AddSymbol(preset.Symbol, preset.Class, preset.Config)
	}
}
