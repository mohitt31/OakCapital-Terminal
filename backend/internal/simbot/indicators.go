package simbot

import "math"

// ──────────────────────────────────────────────────────────────────────────────
// Streaming indicator implementations.
// These operate on a rolling price buffer (slice of recent close prices)
// and return nil when there is insufficient data.
//
// They are direct ports of the frontend's computeSMA/EMA/RSI/MACD/Bollinger
// from useAlphaBot.ts with streaming semantics for the backend runtime.
// ──────────────────────────────────────────────────────────────────────────────

// ComputeSMA returns the Simple Moving Average over the last `period` items.
func ComputeSMA(buf []float64, period int) *float64 {
	n := len(buf)
	if n < period {
		return nil
	}
	sum := 0.0
	for i := n - period; i < n; i++ {
		sum += buf[i]
	}
	result := sum / float64(period)
	return &result
}

// ComputeEMA returns the Exponential Moving Average.
// Computed from scratch each call using the last `period` values as seed.
func ComputeEMA(buf []float64, period int) *float64 {
	n := len(buf)
	if n < period {
		return nil
	}
	k := 2.0 / float64(period+1)
	ema := buf[n-period]
	for i := n - period + 1; i < n; i++ {
		ema = buf[i]*k + ema*(1-k)
	}
	return &ema
}

// ComputeRSI returns the Relative Strength Index over the last `period` changes.
func ComputeRSI(buf []float64, period int) *float64 {
	n := len(buf)
	if n < period+1 {
		return nil
	}
	gains := 0.0
	losses := 0.0
	start := n - period - 1
	for i := start + 1; i < n; i++ {
		diff := buf[i] - buf[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	if avgLoss == 0 {
		result := 100.0
		return &result
	}
	rs := avgGain / avgLoss
	result := 100.0 - 100.0/(1.0+rs)
	return &result
}

// MACDResult holds both MACD line and signal line values.
type MACDResult struct {
	MACDLine   float64
	SignalLine float64
}

// ComputeMACD returns the MACD and Signal lines.
func ComputeMACD(buf []float64, fastPeriod, slowPeriod, signalPeriod int) *MACDResult {
	emaFast := ComputeEMA(buf, fastPeriod)
	emaSlow := ComputeEMA(buf, slowPeriod)
	if emaFast == nil || emaSlow == nil {
		return nil
	}
	macdLine := *emaFast - *emaSlow

	// Approximate signal as SMA of recent MACD values
	// (simplified streaming approach matching the frontend)
	signalBuf := make([]float64, 0, signalPeriod*2)
	startIdx := slowPeriod
	if startIdx > len(buf)-signalPeriod*2 {
		startIdx = len(buf) - signalPeriod*2
	}
	if startIdx < slowPeriod {
		startIdx = slowPeriod
	}
	for i := startIdx; i < len(buf); i++ {
		subBuf := buf[:i+1]
		f := ComputeEMA(subBuf, fastPeriod)
		s := ComputeEMA(subBuf, slowPeriod)
		if f != nil && s != nil {
			signalBuf = append(signalBuf, *f-*s)
		}
	}

	signalLine := macdLine
	if len(signalBuf) >= signalPeriod {
		sma := ComputeSMA(signalBuf, signalPeriod)
		if sma != nil {
			signalLine = *sma
		}
	}

	return &MACDResult{
		MACDLine:   macdLine,
		SignalLine: signalLine,
	}
}

// BollingerResult holds the three Bollinger Band values.
type BollingerResult struct {
	Upper  float64
	Mid    float64
	Lower  float64
}

// ComputeBollinger returns Bollinger Bands (upper, mid, lower).
func ComputeBollinger(buf []float64, period int, stdDevMult float64) *BollingerResult {
	sma := ComputeSMA(buf, period)
	if sma == nil {
		return nil
	}

	n := len(buf)
	variance := 0.0
	for i := n - period; i < n; i++ {
		diff := buf[i] - *sma
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(period))

	return &BollingerResult{
		Upper: *sma + stdDevMult*stdDev,
		Mid:   *sma,
		Lower: *sma - stdDevMult*stdDev,
	}
}
