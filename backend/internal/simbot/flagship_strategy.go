package simbot

// FlagshipStrategyGraph returns the production prebuilt flagship strategy (v2),
// exposed as a one-click server-side bot.
//
// Improvements over the original:
//   - EMA 9/21 instead of 20/50 — significantly faster signal generation,
//     reducing lag so entries are closer to the start of a move.
//   - RSI pivot at 50 instead of 55/45 — avoids buying into already-overbought
//     conditions and selling into already-oversold conditions.
//   - Stop-loss tightened from 1.8% to 1.2% — reduces per-trade max drawdown.
func FlagshipStrategyGraph() StrategyGraph {
	return StrategyGraph{
		Nodes: []BotNode{
			{ID: "price", Type: NodePriceFeed, Params: map[string]interface{}{}, Label: "Price Feed"},
			// Faster EMAs: 9 / 21 (down from original 20 / 50)
			{ID: "emaFast", Type: NodeEMA, Params: map[string]interface{}{"period": 9}, Label: "EMA 9"},
			{ID: "emaSlow", Type: NodeEMA, Params: map[string]interface{}{"period": 21}, Label: "EMA 21"},
			{ID: "cross", Type: NodeCrossover, Params: map[string]interface{}{}, Label: "EMA Cross"},
			// RSI pivot at 50 — buys only when momentum is positive, sells only when negative
			{ID: "rsi", Type: NodeRSI, Params: map[string]interface{}{"period": 14}, Label: "RSI 14"},
			{ID: "rsiLong", Type: NodeThreshold, Params: map[string]interface{}{"operator": ">=", "value": 50}, Label: "RSI Long"},
			{ID: "rsiShort", Type: NodeThreshold, Params: map[string]interface{}{"operator": "<=", "value": 50}, Label: "RSI Short"},
			{ID: "andLong", Type: NodeAND, Params: map[string]interface{}{}, Label: "Long Confirm"},
			{ID: "andShort", Type: NodeAND, Params: map[string]interface{}{}, Label: "Short Confirm"},
			{ID: "buy", Type: NodeMarketBuy, Params: map[string]interface{}{"quantity": 2}, Label: "Buy"},
			{ID: "sell", Type: NodeMarketSell, Params: map[string]interface{}{"quantity": 2}, Label: "Sell"},
			// Tighter stop-loss: 1.2% (down from 1.8%)
			{ID: "stop", Type: NodeStopLoss, Params: map[string]interface{}{"threshold": 1.2, "quantity": 2}, Label: "Stop Loss"},
		},
		Edges: []BotEdge{
			{ID: "e1", FromNode: "emaFast", FromPort: "result", ToNode: "cross", ToPort: "fast"},
			{ID: "e2", FromNode: "emaSlow", FromPort: "result", ToNode: "cross", ToPort: "slow"},
			{ID: "e3", FromNode: "rsi", FromPort: "result", ToNode: "rsiLong", ToPort: "value"},
			{ID: "e4", FromNode: "rsi", FromPort: "result", ToNode: "rsiShort", ToPort: "value"},
			{ID: "e5", FromNode: "cross", FromPort: "crossUp", ToNode: "andLong", ToPort: "a"},
			{ID: "e6", FromNode: "rsiLong", FromPort: "signal", ToNode: "andLong", ToPort: "b"},
			{ID: "e7", FromNode: "cross", FromPort: "crossDown", ToNode: "andShort", ToPort: "a"},
			{ID: "e8", FromNode: "rsiShort", FromPort: "signal", ToNode: "andShort", ToPort: "b"},
			{ID: "e9", FromNode: "andLong", FromPort: "result", ToNode: "buy", ToPort: "trigger"},
			{ID: "e10", FromNode: "andShort", FromPort: "result", ToNode: "sell", ToPort: "trigger"},
			{ID: "e11", FromNode: "andLong", FromPort: "result", ToNode: "stop", ToPort: "trigger"},
		},
	}
}
