package market

// SymbolPreset defines a symbol/class pair with a GBM generator configuration.
type SymbolPreset struct {
	Symbol string
	Class  AssetClass
	Config Config
}

// IndianStockPresets returns the default GBM universe.
// Legacy name is kept for compatibility with callers/tests.
// Universe includes US equities + crypto pairs for international markets.
//
// Key parameter effects:
//   - Sigma:    controls how fast the base price drifts (annualized vol)
//   - Spread:   controls candle body size — tighter = smaller candles
//   - Aggressive: controls how often trades happen (market orders)
//   - TickSize: should match the instrument's minimum quote increment
func IndianStockPresets() []SymbolPreset {
	base := Config{
		Mu:               0.09, // ~9% annual drift (slight upward bias)
		Sigma:            0.20, // ~20% annual vol (typical liquid equity)
		TickIntervalMS:   80,   // ~12.5 ticks/sec
		TickSize:         0.01, // USD cent tick for equities
		MinSpread:        0.02, // 2 ticks
		MaxSpread:        0.12, // 12 ticks
		MinQty:           1,
		MaxQty:           120,
		MaxOrdersPerSide: 5,
		AggressiveRate:   0.08, // ~1 market order/sec — enough for trades, not too many
	}

	withPrice := func(symbol string, class AssetClass, price float64, sigma float64, tick float64, minSpread float64, maxSpread float64, minQty int, maxQty int) SymbolPreset {
		cfg := base
		cfg.InitialPrice = price
		cfg.Sigma = sigma
		return SymbolPreset{
			Symbol: symbol,
			Class:  class,
			Config: cfg,
		}
	}

	// US large-cap equities + major crypto pairs.
	return []SymbolPreset{
		withPrice("AAPL", Stock, 214.30, 0.16, 0.01, 0.02, 0.10, 1, 150),
		withPrice("MSFT", Stock, 437.80, 0.15, 0.01, 0.02, 0.11, 1, 130),
		withPrice("NVDA", Stock, 1223.40, 0.24, 0.01, 0.04, 0.25, 1, 80),
		withPrice("AMZN", Stock, 197.10, 0.20, 0.01, 0.02, 0.15, 1, 160),
		withPrice("TSLA", Stock, 247.80, 0.30, 0.01, 0.04, 0.30, 1, 140),
		withPrice("GOOGL", Stock, 176.20, 0.18, 0.01, 0.02, 0.14, 1, 150),
		withPrice("META", Stock, 502.40, 0.22, 0.01, 0.03, 0.18, 1, 110),
		withPrice("BTCUSD", Crypto, 68250.00, 0.55, 0.50, 1.00, 8.00, 1, 20),
		withPrice("ETHUSD", Crypto, 3225.00, 0.62, 0.10, 0.40, 3.00, 1, 40),
		withPrice("SOLUSD", Crypto, 162.94, 0.80, 0.01, 0.08, 0.70, 5, 220),
	}
}
