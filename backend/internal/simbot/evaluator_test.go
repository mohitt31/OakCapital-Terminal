package simbot

import (
	"encoding/json"
	"testing"
	"time"
)

func TestScalperStrategy(t *testing.T) {
	// A simple scalper strategy JSON from the frontend
	strategyJSON := `{
		"nodes": [
			{"id": "price", "type": "priceFeed", "x": 80, "y": 120, "params": {}},
			{"id": "emaFast", "type": "ema", "x": 320, "y": 60, "params": {"period": 2}},
			{"id": "emaSlow", "type": "ema", "x": 320, "y": 200, "params": {"period": 5}},
			{"id": "cross", "type": "crossover", "x": 560, "y": 130, "params": {}},
			{"id": "buy", "type": "marketBuy", "x": 780, "y": 70, "params": {"quantity": 1}},
			{"id": "sell", "type": "marketSell", "x": 780, "y": 190, "params": {"quantity": 1}}
		],
		"edges": [
			{"id": "e1", "fromNode": "emaFast", "fromPort": "result", "toNode": "cross", "toPort": "fast"},
			{"id": "e2", "fromNode": "emaSlow", "fromPort": "result", "toNode": "cross", "toPort": "slow"},
			{"id": "e3", "fromNode": "cross", "fromPort": "crossUp", "toNode": "buy", "toPort": "trigger"},
			{"id": "e4", "fromNode": "cross", "fromPort": "crossDown", "toNode": "sell", "toPort": "trigger"}
		]
	}`

	var strategy StrategyGraph
	if err := json.Unmarshal([]byte(strategyJSON), &strategy); err != nil {
		t.Fatalf("failed to unmarshal strategy: %v", err)
	}

	evaluator := NewGraphEvaluator(strategy, time.Second)
	// We want to simulate EMA(2) crossing EMA(5).
	// Let's feed prices 100, 100, 100, 100, 100. Then suddenly trend up to 110, then trend down to 90.

	prices := []float64{
		100, 100, 100, 100, 100, 100, 100,
		105, 110, 115, 120, // uptrend
		120, 115, 110, 100, 90, 80, // downtrend
	}

	buyTriggered := false
	sellTriggered := false

	for i, p := range prices {
		// Sleep for 1.1s before the downtrend starts to bypass the 1s OrderCooldown
		if i == 11 {
			time.Sleep(1100 * time.Millisecond)
		}

		actions := evaluator.EvaluateTick(p)

		for _, a := range actions {
			t.Logf("Tick %d (price=%.1f) triggered Action: %s qty %v", i, p, a.Side, a.Quantity)
			if a.Side == "buy" {
				buyTriggered = true
			}
			if a.Side == "sell" {
				sellTriggered = true
			}
		}
	}

	if !buyTriggered {
		t.Error("Expected a BUY action during uptrend but got none")
	}
	if !sellTriggered {
		t.Error("Expected a SELL action during downtrend but got none")
	}
}
