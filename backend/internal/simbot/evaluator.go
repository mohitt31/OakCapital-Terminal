package simbot

import (
	"fmt"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// GraphEvaluator — evaluates a strategy graph on each price tick.
//
// This is a direct port of the frontend's useAlphaBot.ts evaluateTick() logic:
// 1. Topological sort of nodes by edges
// 2. Walk sorted nodes, computing each node's outputs
// 3. Propagate values through edges
// 4. Return OrderActions when action nodes fire
// ──────────────────────────────────────────────────────────────────────────────

// GraphEvaluator holds the strategy graph and all evaluation state across ticks.
type GraphEvaluator struct {
	Strategy StrategyGraph

	// State persisted across ticks
	PriceBuffer       []float64
	PrevValues        map[string]interface{} // for crossover detection
	Cooldown          time.Time              // earliest time the next order can fire
	EntryPrice        float64                // price when bot was started
	LastSide          string                 // last emitted side ("buy" or "sell") to prevent same-side spam
	EvalInterval      time.Duration          // evaluate on candle-close cadence (e.g. 1m)
	lastBucketID      int64
	lastBucketClose   float64
	bucketInitialized bool
}

// NewGraphEvaluator creates a new evaluator for the given strategy graph.
func NewGraphEvaluator(strategy StrategyGraph, evalInterval time.Duration) *GraphEvaluator {
	if evalInterval <= 0 {
		evalInterval = 1 * time.Second
	}
	return &GraphEvaluator{
		Strategy:     strategy,
		PriceBuffer:  make([]float64, 0, MaxPriceBuffer),
		PrevValues:   make(map[string]interface{}),
		EvalInterval: evalInterval,
	}
}

// EvaluateTick runs one tick immediately (legacy behavior, used by tests).
func (ge *GraphEvaluator) EvaluateTick(price float64) []OrderAction {
	return ge.evaluateCore(price)
}

// EvaluateSample ingests a high-frequency sample and only evaluates when the
// configured interval bucket rolls over (candle-close cadence).
// Between evaluations we still track the latest close so indicators have
// one data point per completed candle.
func (ge *GraphEvaluator) EvaluateSample(price float64, ts time.Time) []OrderAction {
	if ts.IsZero() {
		ts = time.Now()
	}
	intervalNanos := ge.EvalInterval.Nanoseconds()
	if intervalNanos <= 0 {
		return ge.evaluateCore(price)
	}
	bucketID := ts.UnixNano() / intervalNanos

	if !ge.bucketInitialized {
		ge.bucketInitialized = true
		ge.lastBucketID = bucketID
		ge.lastBucketClose = price
		return nil
	}

	// Still inside the same bucket — just track latest close.
	if bucketID == ge.lastBucketID {
		ge.lastBucketClose = price
		return nil
	}

	// Bucket rolled: append the closed candle's close price to the price buffer
	// (one entry per candle, not per tick), then run strategy evaluation once.
	closedPrice := ge.lastBucketClose
	ge.PriceBuffer = append(ge.PriceBuffer, closedPrice)
	if len(ge.PriceBuffer) > MaxPriceBuffer {
		ge.PriceBuffer = ge.PriceBuffer[1:]
	}

	ge.lastBucketID = bucketID
	ge.lastBucketClose = price

	return ge.evaluateOnBuffer(closedPrice)
}

// evaluateCore appends price to buffer and runs the graph (used by EvaluateTick).
func (ge *GraphEvaluator) evaluateCore(price float64) []OrderAction {
	ge.PriceBuffer = append(ge.PriceBuffer, price)
	if len(ge.PriceBuffer) > MaxPriceBuffer {
		ge.PriceBuffer = ge.PriceBuffer[1:]
	}
	return ge.evaluateOnBuffer(price)
}

// evaluateOnBuffer runs the strategy graph using the current PriceBuffer.
// Caller must have already appended the candle close to PriceBuffer.
func (ge *GraphEvaluator) evaluateOnBuffer(price float64) []OrderAction {
	if len(ge.Strategy.Nodes) == 0 {
		return nil
	}

	if ge.EntryPrice == 0 {
		ge.EntryPrice = price
	}

	// Topological sort
	sorted := topoSort(ge.Strategy.Nodes, ge.Strategy.Edges)

	// Port values for this tick
	portValues := make(map[string]interface{})

	var actions []OrderAction

	for _, node := range sorted {
		// Gather inputs from edges
		inputs := make(map[string]interface{})
		for _, edge := range ge.Strategy.Edges {
			if edge.ToNode == node.ID {
				key := fmt.Sprintf("%s.%s", edge.FromNode, edge.FromPort)
				if val, ok := portValues[key]; ok {
					inputs[edge.ToPort] = val
				}
			}
		}

		p := node.Params
		buf := ge.PriceBuffer

		switch node.Type {
		case NodePriceFeed:
			portValues[fmt.Sprintf("%s.price", node.ID)] = price

		case NodeSMA:
			period := paramInt(p, "period", 20)
			if v := ComputeSMA(buf, period); v != nil {
				portValues[fmt.Sprintf("%s.result", node.ID)] = *v
			}

		case NodeEMA:
			period := paramInt(p, "period", 12)
			if v := ComputeEMA(buf, period); v != nil {
				portValues[fmt.Sprintf("%s.result", node.ID)] = *v
			}

		case NodeRSI:
			period := paramInt(p, "period", 14)
			if v := ComputeRSI(buf, period); v != nil {
				portValues[fmt.Sprintf("%s.result", node.ID)] = *v
			}

		case NodeMACD:
			fast := paramInt(p, "fastPeriod", 12)
			slow := paramInt(p, "slowPeriod", 26)
			signal := paramInt(p, "signalPeriod", 9)
			if v := ComputeMACD(buf, fast, slow, signal); v != nil {
				portValues[fmt.Sprintf("%s.macdLine", node.ID)] = v.MACDLine
				portValues[fmt.Sprintf("%s.signalLine", node.ID)] = v.SignalLine
			}

		case NodeBollingerBands:
			period := paramInt(p, "period", 20)
			stdDev := paramFloat(p, "stdDev", 2.0)
			if v := ComputeBollinger(buf, period, stdDev); v != nil {
				portValues[fmt.Sprintf("%s.upper", node.ID)] = v.Upper
				portValues[fmt.Sprintf("%s.mid", node.ID)] = v.Mid
				portValues[fmt.Sprintf("%s.lower", node.ID)] = v.Lower
			}

		case NodeCrossover:
			fast, fastOk := toFloat64(inputs["fast"])
			slow, slowOk := toFloat64(inputs["slow"])
			if fastOk && slowOk {
				prevFastKey := fmt.Sprintf("%s.fast", node.ID)
				prevSlowKey := fmt.Sprintf("%s.slow", node.ID)
				prevFast, prevFastOk := toFloat64(ge.PrevValues[prevFastKey])
				prevSlow, prevSlowOk := toFloat64(ge.PrevValues[prevSlowKey])

				crossUp := prevFastOk && prevSlowOk && prevFast <= prevSlow && fast > slow
				crossDown := prevFastOk && prevSlowOk && prevFast >= prevSlow && fast < slow

				portValues[fmt.Sprintf("%s.crossUp", node.ID)] = crossUp
				portValues[fmt.Sprintf("%s.crossDown", node.ID)] = crossDown

				ge.PrevValues[prevFastKey] = fast
				ge.PrevValues[prevSlowKey] = slow
			}

		case NodeThreshold:
			val, valOk := toFloat64(inputs["value"])
			if valOk {
				op := paramString(p, "operator", ">")
				thresh := paramFloat(p, "value", 50)
				var result bool
				switch op {
				case ">":
					result = val > thresh
				case "<":
					result = val < thresh
				case ">=":
					result = val >= thresh
				case "<=":
					result = val <= thresh
				}
				portValues[fmt.Sprintf("%s.signal", node.ID)] = result
			}

		case NodeAND:
			a := toBool(inputs["a"])
			b := toBool(inputs["b"])
			portValues[fmt.Sprintf("%s.result", node.ID)] = a && b

		case NodeOR:
			a := toBool(inputs["a"])
			b := toBool(inputs["b"])
			portValues[fmt.Sprintf("%s.result", node.ID)] = a || b

		case NodeMarketBuy:
			trigger := toBool(inputs["trigger"])
			prevKey := fmt.Sprintf("%s.trigger", node.ID)
			prevTrigger := toBool(ge.PrevValues[prevKey])
			fire := trigger && !prevTrigger
			if fire && ge.LastSide != "buy" && time.Now().After(ge.Cooldown) {
				qty := paramFloat(p, "quantity", 1)
				actions = append(actions, OrderAction{Side: "buy", Quantity: qty})
				ge.LastSide = "buy"
				ge.Cooldown = time.Now().Add(OrderCooldown)
			}
			ge.PrevValues[prevKey] = trigger

		case NodeMarketSell:
			trigger := toBool(inputs["trigger"])
			prevKey := fmt.Sprintf("%s.trigger", node.ID)
			prevTrigger := toBool(ge.PrevValues[prevKey])
			fire := trigger && !prevTrigger
			if fire && ge.LastSide != "sell" && time.Now().After(ge.Cooldown) {
				qty := paramFloat(p, "quantity", 1)
				actions = append(actions, OrderAction{Side: "sell", Quantity: qty})
				ge.LastSide = "sell"
				ge.Cooldown = time.Now().Add(OrderCooldown)
			}
			ge.PrevValues[prevKey] = trigger

		case NodeStopLoss:
			trigger := toBool(inputs["trigger"])
			if trigger && ge.EntryPrice > 0 {
				dropPct := ((ge.EntryPrice - price) / ge.EntryPrice) * 100
				thresh := paramFloat(p, "threshold", 2)
				stopNow := dropPct >= thresh
				prevKey := fmt.Sprintf("%s.stop", node.ID)
				prevStop := toBool(ge.PrevValues[prevKey])
				fire := stopNow && !prevStop
				if fire && time.Now().After(ge.Cooldown) {
					qty := paramFloat(p, "quantity", 1)
					actions = append(actions, OrderAction{Side: "sell", Quantity: qty})
					ge.LastSide = "sell"
					ge.Cooldown = time.Now().Add(StopCooldown)
				}
				ge.PrevValues[prevKey] = stopNow
			} else {
				prevKey := fmt.Sprintf("%s.stop", node.ID)
				ge.PrevValues[prevKey] = false
			}
		}
	}

	return actions
}

// ──────────────────────────────────────────────────────────────────────────────
// Topological sort — Kahn's algorithm, same as the frontend's topoSort().
// ──────────────────────────────────────────────────────────────────────────────

func topoSort(nodes []BotNode, edges []BotEdge) []BotNode {
	inDeg := make(map[string]int)
	adj := make(map[string][]string)

	for _, n := range nodes {
		inDeg[n.ID] = 0
		adj[n.ID] = nil
	}
	for _, e := range edges {
		adj[e.FromNode] = append(adj[e.FromNode], e.ToNode)
		inDeg[e.ToNode]++
	}

	queue := make([]string, 0)
	for id, deg := range inDeg {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	sorted := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, id)
		for _, next := range adj[id] {
			inDeg[next]--
			if inDeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	nodeMap := make(map[string]BotNode, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	result := make([]BotNode, 0, len(sorted))
	for _, id := range sorted {
		if n, ok := nodeMap[id]; ok {
			result = append(result, n)
		}
	}
	return result
}

// ──────────────────────────────────────────────────────────────────────────────
// Parameter extraction helpers
// ──────────────────────────────────────────────────────────────────────────────

func paramInt(params map[string]interface{}, key string, defaultVal int) int {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	default:
		return defaultVal
	}
}

func paramFloat(params map[string]interface{}, key string, defaultVal float64) float64 {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return defaultVal
	}
}

func paramString(params map[string]interface{}, key string, defaultVal string) string {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	if s, ok := v.(string); ok {
		return s
	}
	return defaultVal
}

func toFloat64(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
