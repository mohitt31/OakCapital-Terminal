package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// MarketSimulationConfig holds the configuration for market price simulation.
type MarketSimulationConfig struct {
	ID             int64            `json:"id" db:"id"`
	InstrumentID   int64            `json:"instrument_id" db:"instrument_id"`
	SimulationType SimulationType   `json:"simulation_type" db:"simulation_type"` // GBM, SINE, REPLAY
	GbmMu          *decimal.Decimal `json:"gbm_mu" db:"gbm_mu"`
	GbmSigma       *decimal.Decimal `json:"gbm_sigma" db:"gbm_sigma"`
	GbmSeed        *int64           `json:"gbm_seed" db:"gbm_seed"`     // For reproducible simulations
	LastPrice      *decimal.Decimal `json:"last_price" db:"last_price"` // For warm restarts
	IsActive       bool             `json:"is_active" db:"is_active"`
	UpdatedAt      time.Time        `json:"updated_at" db:"updated_at"`
}
