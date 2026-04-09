package bot

import (
	"fmt"
	"math"
)

// Portfolio tracks position, cash, average entry price, and PnL for one symbol.
// This is the bot-internal tracker, separate from the system-wide internal/portfolio module.
type Portfolio struct {
	Position    float64
	Cash        float64
	AvgPrice    float64
	RealizedPnL float64
}

// NewPortfolio creates a portfolio with default starting cash.
func NewPortfolio() *Portfolio {
	return &Portfolio{Cash: 100000.0}
}

// UpdateOnFill processes a fill event and updates the portfolio state.
func (p *Portfolio) UpdateOnFill(side string, price, qty float64) {
	if side == "buy" {
		p.buy(price, qty)
	} else if side == "sell" {
		p.sell(price, qty)
	}
}

// buy handles a buy fill: closes short first, then opens long.
func (p *Portfolio) buy(price, qty float64) {
	// close short position first
	if p.Position < 0 {
		closingQty := math.Min(qty, math.Abs(p.Position))
		pnl := (p.AvgPrice - price) * closingQty
		p.RealizedPnL += pnl
		p.Position += closingQty
		p.Cash -= price * closingQty
		qty -= closingQty

		if p.Position == 0 {
			p.AvgPrice = 0
		}
	}

	// open or add to long position
	if qty > 0 {
		totalCost := p.AvgPrice*p.Position + price*qty
		p.Position += qty
		p.AvgPrice = totalCost / p.Position
		p.Cash -= price * qty
	}
}

// sell handles a sell fill: closes long first, then opens short.
func (p *Portfolio) sell(price, qty float64) {
	// close long position first
	if p.Position > 0 {
		closingQty := math.Min(qty, p.Position)
		pnl := (price - p.AvgPrice) * closingQty
		p.RealizedPnL += pnl
		p.Position -= closingQty
		p.Cash += price * closingQty
		qty -= closingQty

		if p.Position == 0 {
			p.AvgPrice = 0
		}
	}

	// open or add to short position
	if qty > 0 {
		totalCost := p.AvgPrice*math.Abs(p.Position) + price*qty
		p.Position -= qty
		p.AvgPrice = totalCost / math.Abs(p.Position)
		p.Cash += price * qty
	}
}

// UnrealizedPnL computes mark-to-market PnL at the given mid price.
func (p *Portfolio) UnrealizedPnL(midPrice float64) float64 {
	if p.Position > 0 {
		return (midPrice - p.AvgPrice) * p.Position
	} else if p.Position < 0 {
		return (p.AvgPrice - midPrice) * math.Abs(p.Position)
	}
	return 0
}

// TotalPnL is realized + unrealized.
func (p *Portfolio) TotalPnL(midPrice float64) float64 {
	return p.RealizedPnL + p.UnrealizedPnL(midPrice)
}

// String returns a human-readable summary of the portfolio.
func (p *Portfolio) String() string {
	return fmt.Sprintf("Position: %.0f | AvgPrice: %.2f | Cash: %.2f | RealizedPnL: %.2f",
		p.Position, p.AvgPrice, p.Cash, p.RealizedPnL)
}
