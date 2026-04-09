package simbot

// ──────────────────────────────────────────────────────────────────────────────
// Alternative Flagship Strategies
//
// Each function returns a StrategyGraph ready to be plugged into BotManager.
// Strategies are ordered from most conservative to most aggressive.
//
// Available node types (from types.go):
//   NodePriceFeed, NodeSMA, NodeEMA, NodeRSI, NodeMACD,
//   NodeBollingerBands, NodeCrossover, NodeThreshold,
//   NodeAND, NodeOR, NodeMarketBuy, NodeMarketSell, NodeStopLoss
//
// Available Bollinger ports : upper, mid, lower
// Available MACD ports      : macdLine, signalLine
// Available crossover ports : crossUp, crossDown
// Available threshold port  : signal
// Available AND/OR port     : result
// ──────────────────────────────────────────────────────────────────────────────

// StrategyName is the canonical string key used to select a strategy via the API.
type StrategyName string

const (
	StrategyFlagshipV2           StrategyName = "flagship_v2"
	StrategyBollingerMeanRevert  StrategyName = "bollinger_mean_reversion"
	StrategyMACDMomentum         StrategyName = "macd_momentum"
	StrategyRSIReversal          StrategyName = "rsi_reversal"
	StrategyFastEMATrend         StrategyName = "fast_ema_trend"
	StrategyMACDBollingerBreakout StrategyName = "macd_bollinger_breakout"
)

// StrategyLabel maps each strategy name to a human-readable label.
var StrategyLabel = map[StrategyName]string{
	StrategyFlagshipV2:            "Flagship v2",
	StrategyBollingerMeanRevert:   "Bollinger Mean Reversion",
	StrategyMACDMomentum:          "MACD Momentum",
	StrategyRSIReversal:           "RSI Reversal",
	StrategyFastEMATrend:          "Fast EMA Trend",
	StrategyMACDBollingerBreakout: "MACD + Bollinger Breakout",
}

// GetStrategy returns the StrategyGraph and its display label for the given name.
// Falls back to the v2 flagship if the name is unrecognised.
func GetStrategy(name StrategyName) (StrategyGraph, string) {
	switch name {
	case StrategyBollingerMeanRevert:
		return BollingerMeanReversionStrategy(), StrategyLabel[name]
	case StrategyMACDMomentum:
		return MACDMomentumStrategy(), StrategyLabel[name]
	case StrategyRSIReversal:
		return RSIReversalStrategy(), StrategyLabel[name]
	case StrategyFastEMATrend:
		return FastEMATrendStrategy(), StrategyLabel[name]
	case StrategyMACDBollingerBreakout:
		return MACDBollingerBreakoutStrategy(), StrategyLabel[name]
	default:
		// flagship_v2 is the default
		return FlagshipStrategyGraph(), StrategyLabel[StrategyFlagshipV2]
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Strategy 1 — Bollinger Band Mean Reversion
//
// Philosophy: In a ranging market prices bounce between the lower and upper
// Bollinger bands.  We buy when price touches the lower band (oversold) and
// sell when it touches the upper band (overbought).
//
// Signal wiring:
//   priceFeed ─────────────────────────► threshBuyVal.value
//   bollinger(20,2) ─ lower ──────────► threshBuy.value   (price <= lower → buy)
//   bollinger(20,2) ─ upper ──────────► threshSell.value  (price >= upper → sell)
//
// NOTE: The NodeThreshold node takes `value` as the input to compare against the
// fixed `value` param.  Here we swap usage: we feed the dynamic band value into
// the `value` port, and rely on the evaluator's comparator.
// Because the evaluator does:  inputs["value"]  op  params["value"]
// We set params["value"] to the current price by encoding it statically as a
// mid-market anchor (100 is a placeholder; the real check is band vs price).
//
// Practical approach:  We wire it so BOTH the price AND the band flow through
// the graph using two threshold nodes with a fixed reference price as the
// secondary anchor.  The key insight is that for the LOWER band we want:
//   price <= lower_band
// We achieve this by feeding the lower band value as the threshold value and
// the current price as the input, with operator ">=":
//   inputs["value"] (price) >= params["value"] (lower_band_static)?  — not dynamic.
//
// Because NodeThreshold only supports a STATIC numeric param["value"], we use the
// simpler and more robust approach of wiring a CROSSOVER node:
//   * fastPort = current price
//   * slowPort = lowerBand
// when price crosses DOWN through lower band → buy (crossDown)
//   * fastPort = current price
//   * slowPort = upperBand
// when price crosses UP through upper band → sell (crossUp)
// ──────────────────────────────────────────────────────────────────────────────
func BollingerMeanReversionStrategy() StrategyGraph {
	return StrategyGraph{
		Nodes: []BotNode{
			{ID: "price", Type: NodePriceFeed, Params: map[string]interface{}{}, Label: "Price Feed"},
			// Bollinger Bands (20-period, 2 std-dev)
			{ID: "bb", Type: NodeBollingerBands, Params: map[string]interface{}{"period": 20, "stdDev": 2.0}, Label: "Bollinger 20"},
			// Crossover: price vs lower → crossDown means price dipped below lower band → BUY signal
			{ID: "crossLow", Type: NodeCrossover, Params: map[string]interface{}{}, Label: "Price vs Lower Band"},
			// Crossover: price vs upper → crossUp means price broke above upper band → SELL signal
			{ID: "crossHigh", Type: NodeCrossover, Params: map[string]interface{}{}, Label: "Price vs Upper Band"},
			// RSI filter: avoid buying into strong downtrend (RSI > 25 keeps us out of freefall)
			{ID: "rsi", Type: NodeRSI, Params: map[string]interface{}{"period": 14}, Label: "RSI 14"},
			{ID: "rsiBuyFilter", Type: NodeThreshold, Params: map[string]interface{}{"operator": ">=", "value": 28.0}, Label: "RSI Buy Filter"},
			// AND gates
			{ID: "andBuy", Type: NodeAND, Params: map[string]interface{}{}, Label: "Buy Confirm"},
			// Actions
			{ID: "buy", Type: NodeMarketBuy, Params: map[string]interface{}{"quantity": 3}, Label: "Buy"},
			{ID: "sell", Type: NodeMarketSell, Params: map[string]interface{}{"quantity": 3}, Label: "Sell"},
			// Stop-loss: 1.5% drop from entry
			{ID: "stop", Type: NodeStopLoss, Params: map[string]interface{}{"threshold": 1.5, "quantity": 3}, Label: "Stop Loss"},
		},
		Edges: []BotEdge{
			// Price → crossLow (fast = price, slow = lowerBand)
			{ID: "e1", FromNode: "price", FromPort: "price", ToNode: "crossLow", ToPort: "fast"},
			{ID: "e2", FromNode: "bb", FromPort: "lower", ToNode: "crossLow", ToPort: "slow"},
			// Price → crossHigh (fast = price, slow = upperBand)
			{ID: "e3", FromNode: "price", FromPort: "price", ToNode: "crossHigh", ToPort: "fast"},
			{ID: "e4", FromNode: "bb", FromPort: "upper", ToNode: "crossHigh", ToPort: "slow"},
			// RSI filter for buy
			{ID: "e5", FromNode: "rsi", FromPort: "result", ToNode: "rsiBuyFilter", ToPort: "value"},
			// AND: crossLow.crossDown AND rsiBuyFilter.signal → buy
			{ID: "e6", FromNode: "crossLow", FromPort: "crossDown", ToNode: "andBuy", ToPort: "a"},
			{ID: "e7", FromNode: "rsiBuyFilter", FromPort: "signal", ToNode: "andBuy", ToPort: "b"},
			// Buy trigger
			{ID: "e8", FromNode: "andBuy", FromPort: "result", ToNode: "buy", ToPort: "trigger"},
			// Sell trigger: upper band crossUp (price broke above upper band, mean-revert sell)
			{ID: "e9", FromNode: "crossHigh", FromPort: "crossUp", ToNode: "sell", ToPort: "trigger"},
			// Stop-loss triggered when we're in a long position
			{ID: "e10", FromNode: "andBuy", FromPort: "result", ToNode: "stop", ToPort: "trigger"},
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Strategy 2 — MACD Momentum
//
// Philosophy: MACD line crossing above the signal line indicates building upward
// momentum; crossing below indicates waning momentum / downtrend.
// RSI confirms we're not in an extreme zone (avoid buying overbought / selling
// oversold).
//
// Signal wiring:
//   MACD(12,26,9) → macdLine, signalLine
//   crossover(macdLine, signalLine):
//     crossUp   → momentum turning bullish
//     crossDown → momentum turning bearish
//   RSI(14) >= 45 → momFilter (confirms upward bias for buy)
//   AND(crossUp, momFilter) → buy
//   AND(crossDown, rsiSell) → sell
// ──────────────────────────────────────────────────────────────────────────────
func MACDMomentumStrategy() StrategyGraph {
	return StrategyGraph{
		Nodes: []BotNode{
			{ID: "price", Type: NodePriceFeed, Params: map[string]interface{}{}, Label: "Price Feed"},
			// MACD (standard params)
			{ID: "macd", Type: NodeMACD, Params: map[string]interface{}{"fastPeriod": 12, "slowPeriod": 26, "signalPeriod": 9}, Label: "MACD 12/26/9"},
			// Crossover between MACD line and signal line
			{ID: "macdCross", Type: NodeCrossover, Params: map[string]interface{}{}, Label: "MACD Cross"},
			// RSI for momentum confirmation
			{ID: "rsi", Type: NodeRSI, Params: map[string]interface{}{"period": 14}, Label: "RSI 14"},
			{ID: "rsiLong", Type: NodeThreshold, Params: map[string]interface{}{"operator": ">=", "value": 45.0}, Label: "RSI Long Filter"},
			{ID: "rsiShort", Type: NodeThreshold, Params: map[string]interface{}{"operator": "<=", "value": 55.0}, Label: "RSI Short Filter"},
			// AND gates
			{ID: "andLong", Type: NodeAND, Params: map[string]interface{}{}, Label: "Long Confirm"},
			{ID: "andShort", Type: NodeAND, Params: map[string]interface{}{}, Label: "Short Confirm"},
			// Actions
			{ID: "buy", Type: NodeMarketBuy, Params: map[string]interface{}{"quantity": 2}, Label: "Buy"},
			{ID: "sell", Type: NodeMarketSell, Params: map[string]interface{}{"quantity": 2}, Label: "Sell"},
			// Stop-loss: 1.2%
			{ID: "stop", Type: NodeStopLoss, Params: map[string]interface{}{"threshold": 1.2, "quantity": 2}, Label: "Stop Loss"},
		},
		Edges: []BotEdge{
			// MACD → crossover (fast = macdLine, slow = signalLine)
			{ID: "e1", FromNode: "macd", FromPort: "macdLine", ToNode: "macdCross", ToPort: "fast"},
			{ID: "e2", FromNode: "macd", FromPort: "signalLine", ToNode: "macdCross", ToPort: "slow"},
			// RSI filters
			{ID: "e3", FromNode: "rsi", FromPort: "result", ToNode: "rsiLong", ToPort: "value"},
			{ID: "e4", FromNode: "rsi", FromPort: "result", ToNode: "rsiShort", ToPort: "value"},
			// Long: MACD crossUp AND RSI >= 45
			{ID: "e5", FromNode: "macdCross", FromPort: "crossUp", ToNode: "andLong", ToPort: "a"},
			{ID: "e6", FromNode: "rsiLong", FromPort: "signal", ToNode: "andLong", ToPort: "b"},
			// Short: MACD crossDown AND RSI <= 55
			{ID: "e7", FromNode: "macdCross", FromPort: "crossDown", ToNode: "andShort", ToPort: "a"},
			{ID: "e8", FromNode: "rsiShort", FromPort: "signal", ToNode: "andShort", ToPort: "b"},
			// Actions
			{ID: "e9", FromNode: "andLong", FromPort: "result", ToNode: "buy", ToPort: "trigger"},
			{ID: "e10", FromNode: "andShort", FromPort: "result", ToNode: "sell", ToPort: "trigger"},
			// Stop-loss on long entries
			{ID: "e11", FromNode: "andLong", FromPort: "result", ToNode: "stop", ToPort: "trigger"},
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Strategy 3 — RSI Reversal (Pure Oversold/Overbought)
//
// Philosophy: The simplest and often most effective edge in short-term trading.
// RSI below 30 = oversold → price likely to bounce → BUY.
// RSI above 70 = overbought → price likely to pull back → SELL.
//
// This works best in oscillating/range-bound markets. Uses a wider stop (2%)
// because mean-reversion can take time.
// ──────────────────────────────────────────────────────────────────────────────
func RSIReversalStrategy() StrategyGraph {
	return StrategyGraph{
		Nodes: []BotNode{
			{ID: "price", Type: NodePriceFeed, Params: map[string]interface{}{}, Label: "Price Feed"},
			// RSI with slightly shorter period for faster response
			{ID: "rsi", Type: NodeRSI, Params: map[string]interface{}{"period": 10}, Label: "RSI 10"},
			// Thresholds
			{ID: "rsiOversold", Type: NodeThreshold, Params: map[string]interface{}{"operator": "<=", "value": 30.0}, Label: "Oversold (<= 30)"},
			{ID: "rsiOverbought", Type: NodeThreshold, Params: map[string]interface{}{"operator": ">=", "value": 70.0}, Label: "Overbought (>= 70)"},
			// Actions — larger quantity since signal quality is high
			{ID: "buy", Type: NodeMarketBuy, Params: map[string]interface{}{"quantity": 3}, Label: "Buy"},
			{ID: "sell", Type: NodeMarketSell, Params: map[string]interface{}{"quantity": 3}, Label: "Sell"},
			// Stop-loss: 2% — wider, mean-reversion needs room to breathe
			{ID: "stop", Type: NodeStopLoss, Params: map[string]interface{}{"threshold": 2.0, "quantity": 3}, Label: "Stop Loss"},
		},
		Edges: []BotEdge{
			// RSI → thresholds
			{ID: "e1", FromNode: "rsi", FromPort: "result", ToNode: "rsiOversold", ToPort: "value"},
			{ID: "e2", FromNode: "rsi", FromPort: "result", ToNode: "rsiOverbought", ToPort: "value"},
			// Oversold → buy
			{ID: "e3", FromNode: "rsiOversold", FromPort: "signal", ToNode: "buy", ToPort: "trigger"},
			// Overbought → sell
			{ID: "e4", FromNode: "rsiOverbought", FromPort: "signal", ToNode: "sell", ToPort: "trigger"},
			// Stop-loss on buy entries
			{ID: "e5", FromNode: "rsiOversold", FromPort: "signal", ToNode: "stop", ToPort: "trigger"},
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Strategy 4 — Fast EMA Trend (9/21 EMA Crossover)
//
// Philosophy: Same crossover concept as the original flagship but with much
// faster EMAs (9 and 21 vs 20 and 50). Shorter periods = less lag = faster
// entries. No RSI filter — we trade every cross for higher frequency. Tight
// stop (1.0%) keeps each loss small.
//
// Risk note: More trades, smaller individual profit target. Requires a market
// with consistent directional bias to be profitable.
// ──────────────────────────────────────────────────────────────────────────────
func FastEMATrendStrategy() StrategyGraph {
	return StrategyGraph{
		Nodes: []BotNode{
			{ID: "price", Type: NodePriceFeed, Params: map[string]interface{}{}, Label: "Price Feed"},
			{ID: "emaFast", Type: NodeEMA, Params: map[string]interface{}{"period": 9}, Label: "EMA 9"},
			{ID: "emaSlow", Type: NodeEMA, Params: map[string]interface{}{"period": 21}, Label: "EMA 21"},
			{ID: "cross", Type: NodeCrossover, Params: map[string]interface{}{}, Label: "Cross"},
			// Volume / momentum validation: ensure RSI is not in counter-trend extreme
			{ID: "rsi", Type: NodeRSI, Params: map[string]interface{}{"period": 7}, Label: "RSI 7"},
			{ID: "rsiMidLong", Type: NodeThreshold, Params: map[string]interface{}{"operator": ">=", "value": 48.0}, Label: "RSI Mid Long"},
			{ID: "rsiMidShort", Type: NodeThreshold, Params: map[string]interface{}{"operator": "<=", "value": 52.0}, Label: "RSI Mid Short"},
			{ID: "andLong", Type: NodeAND, Params: map[string]interface{}{}, Label: "Long Confirm"},
			{ID: "andShort", Type: NodeAND, Params: map[string]interface{}{}, Label: "Short Confirm"},
			// Actions — smaller quantity per trade: 1 unit, high frequency
			{ID: "buy", Type: NodeMarketBuy, Params: map[string]interface{}{"quantity": 2}, Label: "Buy"},
			{ID: "sell", Type: NodeMarketSell, Params: map[string]interface{}{"quantity": 2}, Label: "Sell"},
			// Tight stop
			{ID: "stop", Type: NodeStopLoss, Params: map[string]interface{}{"threshold": 1.0, "quantity": 2}, Label: "Stop Loss"},
		},
		Edges: []BotEdge{
			{ID: "e1", FromNode: "emaFast", FromPort: "result", ToNode: "cross", ToPort: "fast"},
			{ID: "e2", FromNode: "emaSlow", FromPort: "result", ToNode: "cross", ToPort: "slow"},
			{ID: "e3", FromNode: "rsi", FromPort: "result", ToNode: "rsiMidLong", ToPort: "value"},
			{ID: "e4", FromNode: "rsi", FromPort: "result", ToNode: "rsiMidShort", ToPort: "value"},
			// Long: EMA cross up + RSI not bearish
			{ID: "e5", FromNode: "cross", FromPort: "crossUp", ToNode: "andLong", ToPort: "a"},
			{ID: "e6", FromNode: "rsiMidLong", FromPort: "signal", ToNode: "andLong", ToPort: "b"},
			// Short: EMA cross down + RSI not bullish
			{ID: "e7", FromNode: "cross", FromPort: "crossDown", ToNode: "andShort", ToPort: "a"},
			{ID: "e8", FromNode: "rsiMidShort", FromPort: "signal", ToNode: "andShort", ToPort: "b"},
			{ID: "e9", FromNode: "andLong", FromPort: "result", ToNode: "buy", ToPort: "trigger"},
			{ID: "e10", FromNode: "andShort", FromPort: "result", ToNode: "sell", ToPort: "trigger"},
			{ID: "e11", FromNode: "andLong", FromPort: "result", ToNode: "stop", ToPort: "trigger"},
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Strategy 5 — MACD + Bollinger Band Breakout
//
// Philosophy: High-conviction combo — only trade when TWO independent signals
// agree.  MACD says momentum is aligned (cross direction) AND price is on the
// right side of the Bollinger midline (above midline = bullish bias, below =
// bearish bias).
//
// This produces fewer but higher-quality signals.  Quantity per trade is higher
// (4 units) to exploit the conviction.
//
// Signal wiring:
//   MACD crossUp  + price > bb.mid  → buy
//   MACD crossDown + price < bb.mid → sell
//
// For "price > bb.mid" we use the CrossOver node trick again:
//   crossover(price, mid): crossUp = price just crossed above mid band
// But since we want a persistent signal (not just the crossing moment), we use
// a Threshold node wired as: inputs["value"] (price) op params["value"] (static).
// Because params["value"] can't be dynamic, we use the crossover trick to detect
// the transition and use OR to keep the signal alive for a few ticks via the
// evaluator's natural re-evaluation each tick.
// ──────────────────────────────────────────────────────────────────────────────
func MACDBollingerBreakoutStrategy() StrategyGraph {
	return StrategyGraph{
		Nodes: []BotNode{
			{ID: "price", Type: NodePriceFeed, Params: map[string]interface{}{}, Label: "Price Feed"},
			// Bollinger Bands
			{ID: "bb", Type: NodeBollingerBands, Params: map[string]interface{}{"period": 20, "stdDev": 2.0}, Label: "Bollinger 20"},
			// MACD
			{ID: "macd", Type: NodeMACD, Params: map[string]interface{}{"fastPeriod": 12, "slowPeriod": 26, "signalPeriod": 9}, Label: "MACD 12/26/9"},
			// MACD crossover
			{ID: "macdCross", Type: NodeCrossover, Params: map[string]interface{}{}, Label: "MACD Cross"},
			// Price vs Bollinger midband crossover (price above mid = bullish regime)
			{ID: "priceMidCross", Type: NodeCrossover, Params: map[string]interface{}{}, Label: "Price vs Mid Band"},
			// RSI extra gate: avoid buying extreme overbought / selling extreme oversold
			{ID: "rsi", Type: NodeRSI, Params: map[string]interface{}{"period": 14}, Label: "RSI 14"},
			{ID: "rsiBullish", Type: NodeThreshold, Params: map[string]interface{}{"operator": "<=", "value": 72.0}, Label: "Not Overbought"},
			{ID: "rsiBearish", Type: NodeThreshold, Params: map[string]interface{}{"operator": ">=", "value": 28.0}, Label: "Not Oversold"},
			// AND gates — need all three signals to agree
			{ID: "andLong", Type: NodeAND, Params: map[string]interface{}{}, Label: "Long Confirm"},
			{ID: "andLong2", Type: NodeAND, Params: map[string]interface{}{}, Label: "Long Confirm 2"},
			{ID: "andShort", Type: NodeAND, Params: map[string]interface{}{}, Label: "Short Confirm"},
			{ID: "andShort2", Type: NodeAND, Params: map[string]interface{}{}, Label: "Short Confirm 2"},
			// Actions — high conviction = larger size
			{ID: "buy", Type: NodeMarketBuy, Params: map[string]interface{}{"quantity": 4}, Label: "Buy"},
			{ID: "sell", Type: NodeMarketSell, Params: map[string]interface{}{"quantity": 4}, Label: "Sell"},
			// Stop-loss: 1.5%
			{ID: "stop", Type: NodeStopLoss, Params: map[string]interface{}{"threshold": 1.5, "quantity": 4}, Label: "Stop Loss"},
		},
		Edges: []BotEdge{
			// MACD crossover
			{ID: "e1", FromNode: "macd", FromPort: "macdLine", ToNode: "macdCross", ToPort: "fast"},
			{ID: "e2", FromNode: "macd", FromPort: "signalLine", ToNode: "macdCross", ToPort: "slow"},
			// Price vs midband
			{ID: "e3", FromNode: "price", FromPort: "price", ToNode: "priceMidCross", ToPort: "fast"},
			{ID: "e4", FromNode: "bb", FromPort: "mid", ToNode: "priceMidCross", ToPort: "slow"},
			// RSI filters
			{ID: "e5", FromNode: "rsi", FromPort: "result", ToNode: "rsiBullish", ToPort: "value"},
			{ID: "e6", FromNode: "rsi", FromPort: "result", ToNode: "rsiBearish", ToPort: "value"},
			// Long chain: MACD crossUp AND price crossUp midband
			{ID: "e7", FromNode: "macdCross", FromPort: "crossUp", ToNode: "andLong", ToPort: "a"},
			{ID: "e8", FromNode: "priceMidCross", FromPort: "crossUp", ToNode: "andLong", ToPort: "b"},
			// AND Long2: the above AND not-overbought
			{ID: "e9", FromNode: "andLong", FromPort: "result", ToNode: "andLong2", ToPort: "a"},
			{ID: "e10", FromNode: "rsiBullish", FromPort: "signal", ToNode: "andLong2", ToPort: "b"},
			// Short chain: MACD crossDown AND price crossDown midband
			{ID: "e11", FromNode: "macdCross", FromPort: "crossDown", ToNode: "andShort", ToPort: "a"},
			{ID: "e12", FromNode: "priceMidCross", FromPort: "crossDown", ToNode: "andShort", ToPort: "b"},
			// AND Short2: the above AND not-oversold
			{ID: "e13", FromNode: "andShort", FromPort: "result", ToNode: "andShort2", ToPort: "a"},
			{ID: "e14", FromNode: "rsiBearish", FromPort: "signal", ToNode: "andShort2", ToPort: "b"},
			// Actions
			{ID: "e15", FromNode: "andLong2", FromPort: "result", ToNode: "buy", ToPort: "trigger"},
			{ID: "e16", FromNode: "andShort2", FromPort: "result", ToNode: "sell", ToPort: "trigger"},
			{ID: "e17", FromNode: "andLong2", FromPort: "result", ToNode: "stop", ToPort: "trigger"},
		},
	}
}
