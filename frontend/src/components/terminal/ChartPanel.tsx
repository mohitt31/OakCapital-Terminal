import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react'
import {
  ArrowUpRight,
  Camera,
  ChevronDown,
  ChevronUp,
  Circle,
  Crosshair,
  Eraser,
  Expand,
  Eye,
  EyeOff,
  GitCommitHorizontal,
  GripVertical,
  Magnet,
  Minus,
  MoveHorizontal,
  MousePointer2,
  PenTool,
  Plus,
  Redo2,
  RotateCcw,
  Ruler,
  Search,
  Slash,
  SplitSquareVertical,
  Square,
  Trash2,
  TrendingUp,
  Type,
  Undo2,
  X,
  ZoomIn,
} from 'lucide-react'
import { CandlestickSeries, LineSeries, createChart, type IChartApi, type ISeriesApi, type UTCTimestamp } from 'lightweight-charts'
import type { CandlePoint } from '../../types/market'
import { monoClass, scrollClass } from './constants'

// ─── lightweight-charts layout constants ────────────────────────────────────
// These match the default lightweight-charts right price-scale width and the
// time-scale height so we can clip drawings to the actual plot area.
const PRICE_SCALE_WIDTH = 65   // px – right price-scale gutter
const TIME_SCALE_HEIGHT = 22   // px – bottom time-scale gutter

type IntervalKey = '1s' | '5s' | '15s' | '30s' | '1m' | '3m' | '5m' | '15m'
type ToolKey = 'cursor' | 'cross' | 'line' | 'hline' | 'vline' | 'ray' | 'xline' | 'fib' | 'rect' | 'brush' | 'text' | 'arrow' | 'measure' | 'move' | 'magnet' | 'eraser' | 'zoom'
type IndicatorTab = 'all' | 'favorites' | 'builtins'
type StudyKey =
  | 'ema9'
  | 'ema21'
  | 'ema50'
  | 'ema200'
  | 'sma20'
  | 'sma50'
  | 'wma20'
  | 'vwap'
  | 'bbUpper'
  | 'bbMid'
  | 'bbLower'
  | 'rsi14'
  | 'macd'
  | 'atr14'
  | 'stoch14'
  | 'supertrend'
  | 'avgprice'
  | 'alma'
  | 'adx'
  | 'aroonUp'
  | 'accdist'
  | 'hi52'
  | 'lo52'

type IndicatorItem = {
  id: string
  label: string
  category: string
  key?: StudyKey
}

type Props = {
  symbol: string
  secondarySymbol?: string
  assets: string[]
  lastPrice: number
  tickDirection: 1 | -1 | 0
  candles1s: CandlePoint[]
  candles5s: CandlePoint[]
  secondaryCandles1s?: CandlePoint[]
  secondaryCandles5s?: CandlePoint[]
  onSelectAsset?: (symbol: string) => void
  onSelectSecondaryAsset?: (symbol: string) => void
  onQuickTrade?: (action: 'buy' | 'sell', quantity: number, orderType: 'limit' | 'market', limitPrice: number) => void
}

type StudySeriesMap = Partial<Record<StudyKey, ISeriesApi<'Line'>>>
type LowerPaneKey = 'none' | 'rsi' | 'macd' | 'atr' | 'stoch' | 'adx' | 'aroon' | 'accdist'
type LineKind = 'line' | 'hline' | 'vline' | 'ray' | 'xline' | 'arrow'
type OverlayLine = { id: number; kind: LineKind; x1: number; y1: number; x2: number; y2: number; logical1?: number; price1?: number; logical2?: number; price2?: number }
type OverlayRect = { id: number; x1: number; y1: number; x2: number; y2: number; logical1?: number; price1?: number; logical2?: number; price2?: number }
type OverlayFib = { id: number; x1: number; y1: number; x2: number; y2: number; logical1?: number; price1?: number; logical2?: number; price2?: number }
type OverlayLabel = { id: number; x: number; y: number; logical?: number; price?: number; text: string }
type OverlayPoint = { x: number; y: number; logical?: number; price?: number }
type OverlayBrush = { id: number; points: OverlayPoint[] }

const timeframeToSeconds: Record<IntervalKey, number> = {
  '1s': 1,
  '5s': 5,
  '15s': 15,
  '30s': 30,
  '1m': 60,
  '3m': 180,
  '5m': 300,
  '15m': 900,
}

const timeframeOptions: IntervalKey[] = ['1s', '5s', '15s', '30s', '1m', '3m', '5m', '15m']

const overlayStudies: { key: StudyKey; label: string; color: string }[] = [
  { key: 'ema9', label: 'EMA 9', color: '#F59E0B' },
  { key: 'ema21', label: 'EMA 21', color: '#60A5FA' },
  { key: 'ema50', label: 'EMA 50', color: '#A78BFA' },
  { key: 'ema200', label: 'EMA 200', color: '#F97316' },
  { key: 'sma20', label: 'SMA 20', color: '#22D3EE' },
  { key: 'sma50', label: 'SMA 50', color: '#34D399' },
  { key: 'wma20', label: 'WMA 20', color: '#F472B6' },
  { key: 'vwap', label: 'VWAP', color: '#FCD34D' },
  { key: 'bbUpper', label: 'BB Upper', color: '#93C5FD' },
  { key: 'bbMid', label: 'BB Mid', color: '#A3A3A3' },
  { key: 'bbLower', label: 'BB Lower', color: '#93C5FD' },
  { key: 'supertrend', label: 'Supertrend', color: '#E879F9' },
  { key: 'avgprice', label: 'Avg Price', color: '#FB923C' },
  { key: 'alma', label: 'ALMA', color: '#38BDF8' },
  { key: 'hi52', label: '52W High', color: '#4ADE80' },
  { key: 'lo52', label: '52W Low', color: '#F87171' },
]

const indicatorCatalog: IndicatorItem[] = [
  { id: 'ema9', label: 'Exponential Moving Average (9)', category: 'Moving Averages', key: 'ema9' },
  { id: 'ema21', label: 'Exponential Moving Average (21)', category: 'Moving Averages', key: 'ema21' },
  { id: 'ema50', label: 'Exponential Moving Average (50)', category: 'Moving Averages', key: 'ema50' },
  { id: 'ema200', label: 'Exponential Moving Average (200)', category: 'Moving Averages', key: 'ema200' },
  { id: 'sma20', label: 'Simple Moving Average (20)', category: 'Moving Averages', key: 'sma20' },
  { id: 'sma50', label: 'Simple Moving Average (50)', category: 'Moving Averages', key: 'sma50' },
  { id: 'wma20', label: 'Weighted Moving Average (20)', category: 'Moving Averages', key: 'wma20' },
  { id: 'vwap', label: 'VWAP', category: 'Volume', key: 'vwap' },
  { id: 'bbands', label: 'Bollinger Bands', category: 'Volatility', key: 'bbMid' },
  { id: 'rsi14', label: 'Relative Strength Index (14)', category: 'Oscillators', key: 'rsi14' },
  { id: 'macd', label: 'MACD (12,26,9)', category: 'Oscillators', key: 'macd' },
  { id: 'atr14', label: 'Average True Range (14)', category: 'Volatility', key: 'atr14' },
  { id: 'stoch14', label: 'Stochastic (14,3)', category: 'Oscillators', key: 'stoch14' },
  { id: 'supertrend', label: 'Supertrend (10,3)', category: 'Trend', key: 'supertrend' },
  { id: '52w', label: '52 Week High/Low', category: 'Price Action', key: 'hi52' },
  { id: 'accdist', label: 'Accumulation/Distribution', category: 'Volume', key: 'accdist' },
  { id: 'alma', label: 'Arnaud Legoux Moving Average', category: 'Moving Averages', key: 'alma' },
  { id: 'aroon', label: 'Aroon', category: 'Trend', key: 'aroonUp' },
  { id: 'adx', label: 'Average Directional Index', category: 'Trend', key: 'adx' },
  { id: 'avgprice', label: 'Average Price', category: 'Price Action', key: 'avgprice' },
]

const toCandleSeries = (candles: CandlePoint[]) =>
  candles.map((c) => ({
    time: c.time as UTCTimestamp,
    open: c.open,
    high: c.high,
    low: c.low,
    close: c.close,
  }))

const aggregateCandles = (candles: CandlePoint[], bucketSeconds: number) => {
  if (bucketSeconds <= 1) return candles
  const aggregated: CandlePoint[] = []
  let current: CandlePoint | null = null
  for (const candle of candles) {
    const bucketTime = Math.floor(candle.time / bucketSeconds) * bucketSeconds
    if (!current || current.time !== bucketTime) {
      if (current) aggregated.push(current)
      current = { time: bucketTime, open: candle.open, high: candle.high, low: candle.low, close: candle.close }
      continue
    }
    current.high = Math.max(current.high, candle.high)
    current.low = Math.min(current.low, candle.low)
    current.close = candle.close
  }
  if (current) aggregated.push(current)
  return aggregated
}

const ema = (candles: CandlePoint[], period: number) => {
  if (candles.length === 0) return []
  const k = 2 / (period + 1)
  let value = candles[0].close
  return candles.map((candle, index) => {
    value = index === 0 ? candle.close : candle.close * k + value * (1 - k)
    return { time: candle.time as UTCTimestamp, value: Number(value.toFixed(2)) }
  })
}

const sma = (candles: CandlePoint[], period: number) => {
  const output: { time: UTCTimestamp; value: number }[] = []
  let sum = 0
  for (let i = 0; i < candles.length; i += 1) {
    sum += candles[i].close
    if (i >= period) sum -= candles[i - period].close
    const value = i + 1 >= period ? sum / period : candles[i].close
    output.push({ time: candles[i].time as UTCTimestamp, value: Number(value.toFixed(2)) })
  }
  return output
}

const wma = (candles: CandlePoint[], period: number) => {
  const output: { time: UTCTimestamp; value: number }[] = []
  const denominator = (period * (period + 1)) / 2
  for (let i = 0; i < candles.length; i += 1) {
    if (i + 1 < period) {
      output.push({ time: candles[i].time as UTCTimestamp, value: candles[i].close })
      continue
    }
    let weighted = 0
    for (let j = 0; j < period; j += 1) weighted += candles[i - j].close * (period - j)
    output.push({ time: candles[i].time as UTCTimestamp, value: Number((weighted / denominator).toFixed(2)) })
  }
  return output
}

const bbands = (candles: CandlePoint[]) => {
  const mid: { time: UTCTimestamp; value: number }[] = []
  const upper: { time: UTCTimestamp; value: number }[] = []
  const lower: { time: UTCTimestamp; value: number }[] = []
  for (let i = 0; i < candles.length; i += 1) {
    const sample = candles.slice(Math.max(0, i - 19), i + 1).map((c) => c.close)
    const mean = sample.reduce((a, b) => a + b, 0) / sample.length
    const std = Math.sqrt(sample.reduce((a, b) => a + (b - mean) ** 2, 0) / sample.length)
    mid.push({ time: candles[i].time as UTCTimestamp, value: Number(mean.toFixed(2)) })
    upper.push({ time: candles[i].time as UTCTimestamp, value: Number((mean + 2 * std).toFixed(2)) })
    lower.push({ time: candles[i].time as UTCTimestamp, value: Number((mean - 2 * std).toFixed(2)) })
  }
  return { mid, upper, lower }
}

const vwap = (candles: CandlePoint[]) => {
  let cumulativePV = 0
  let cumulativeVol = 0
  return candles.map((candle) => {
    const typical = (candle.high + candle.low + candle.close) / 3
    const syntheticVolume = Math.max(1, Math.round((candle.high - candle.low) * 1000))
    cumulativePV += typical * syntheticVolume
    cumulativeVol += syntheticVolume
    return { time: candle.time as UTCTimestamp, value: Number((cumulativePV / Math.max(1, cumulativeVol)).toFixed(2)) }
  })
}

const rsi = (candles: CandlePoint[]) => {
  if (candles.length < 2) return 50
  let gains = 0, losses = 0
  const start = Math.max(1, candles.length - 15)
  for (let i = start; i < candles.length; i += 1) {
    const delta = candles[i].close - candles[i - 1].close
    if (delta >= 0) gains += delta
    else losses += Math.abs(delta)
  }
  if (losses === 0) return 100
  const rs = (gains / 14) / (losses / 14)
  return Number((100 - 100 / (1 + rs)).toFixed(2))
}

const macd = (candles: CandlePoint[]) => {
  const ema12 = ema(candles, 12).map((p) => p.value)
  const ema26 = ema(candles, 26).map((p) => p.value)
  const line = ema12.map((v, i) => v - (ema26[i] ?? v))
  if (line.length === 0) return { line: 0, signal: 0, hist: 0 }
  let signal = line[0]
  const k = 2 / 10
  for (let i = 1; i < line.length; i += 1) signal = line[i] * k + signal * (1 - k)
  const latest = line[line.length - 1]
  return { line: Number(latest.toFixed(2)), signal: Number(signal.toFixed(2)), hist: Number((latest - signal).toFixed(2)) }
}

const atr = (candles: CandlePoint[]) => {
  if (candles.length < 2) return 0
  const trValues: number[] = []
  for (let i = 1; i < candles.length; i += 1) {
    trValues.push(Math.max(candles[i].high - candles[i].low, Math.abs(candles[i].high - candles[i - 1].close), Math.abs(candles[i].low - candles[i - 1].close)))
  }
  const sample = trValues.slice(-14)
  return Number((sample.reduce((a, b) => a + b, 0) / Math.max(1, sample.length)).toFixed(2))
}

const stoch = (candles: CandlePoint[]) => {
  if (candles.length === 0) return { k: 0, d: 0 }
  const values: number[] = []
  for (let i = 0; i < candles.length; i += 1) {
    const sample = candles.slice(Math.max(0, i - 13), i + 1)
    const hh = Math.max(...sample.map((c) => c.high))
    const ll = Math.min(...sample.map((c) => c.low))
    values.push(hh === ll ? 50 : ((candles[i].close - ll) / (hh - ll)) * 100)
  }
  const k = values[values.length - 1]
  const d = values.slice(-3).reduce((a, b) => a + b, 0) / Math.max(1, values.slice(-3).length)
  return { k: Number(k.toFixed(2)), d: Number(d.toFixed(2)) }
}

const supertrend = (candles: CandlePoint[]) => {
  if (candles.length === 0) return { value: 0, direction: 'UP' as 'UP' | 'DOWN' }
  const latest = candles[candles.length - 1]
  const latestAtr = atr(candles)
  const mid = (latest.high + latest.low) / 2
  const direction: 'UP' | 'DOWN' = latest.close >= mid ? 'UP' : 'DOWN'
  return { value: Number((direction === 'UP' ? mid - 3 * latestAtr : mid + 3 * latestAtr).toFixed(2)), direction }
}

const rsiSeries = (candles: CandlePoint[], period = 14) => {
  if (candles.length === 0) return []
  const values: { time: UTCTimestamp; value: number }[] = []
  for (let i = 0; i < candles.length; i += 1) {
    if (i === 0) { values.push({ time: candles[i].time as UTCTimestamp, value: 50 }); continue }
    let gains = 0, losses = 0
    const start = Math.max(1, i - period + 1)
    for (let j = start; j <= i; j += 1) {
      const delta = candles[j].close - candles[j - 1].close
      if (delta >= 0) gains += delta
      else losses += Math.abs(delta)
    }
    const rs = losses === 0 ? 100 : (gains / period) / (losses / period)
    const value = losses === 0 ? 100 : 100 - 100 / (1 + rs)
    values.push({ time: candles[i].time as UTCTimestamp, value: Number(value.toFixed(2)) })
  }
  return values
}

const macdSeries = (candles: CandlePoint[]) => {
  const fast = ema(candles, 12)
  const slow = ema(candles, 26)
  return fast.map((p, i) => ({ time: p.time, value: Number((p.value - (slow[i]?.value ?? p.value)).toFixed(2)) }))
}

const supertrendSeries = (candles: CandlePoint[]) => {
  if (candles.length < 2) return []
  const period = 10, mult = 3
  const output: { time: UTCTimestamp; value: number }[] = []
  let prevUpper = 0, prevLower = 0, trend = 1
  for (let i = 0; i < candles.length; i++) {
    const hl2 = (candles[i].high + candles[i].low) / 2
    const trs: number[] = []
    for (let j = Math.max(1, i - period + 1); j <= i; j++) {
      trs.push(Math.max(candles[j].high - candles[j].low, Math.abs(candles[j].high - candles[j - 1].close), Math.abs(candles[j].low - candles[j - 1].close)))
    }
    if (i === 0) trs.push(candles[0].high - candles[0].low)
    const atrVal = trs.reduce((a, b) => a + b, 0) / trs.length
    let upper = hl2 + mult * atrVal
    let lower = hl2 - mult * atrVal
    if (i > 0) {
      upper = upper < prevUpper || candles[i - 1].close > prevUpper ? upper : prevUpper
      lower = lower > prevLower || candles[i - 1].close < prevLower ? lower : prevLower
    }
    if (i === 0) trend = 1
    else if (candles[i].close > prevUpper) trend = 1
    else if (candles[i].close < prevLower) trend = -1
    output.push({ time: candles[i].time as UTCTimestamp, value: Number((trend === 1 ? lower : upper).toFixed(2)) })
    prevUpper = upper
    prevLower = lower
  }
  return output
}

const avgPriceSeries = (candles: CandlePoint[]) =>
  candles.map((c) => ({ time: c.time as UTCTimestamp, value: Number(((c.high + c.low + c.close) / 3).toFixed(2)) }))

const almaSeries = (candles: CandlePoint[], windowSize = 20, sigma = 6, offset = 0.85) => {
  const output: { time: UTCTimestamp; value: number }[] = []
  const m = offset * (windowSize - 1)
  const s = windowSize / sigma
  for (let i = 0; i < candles.length; i++) {
    if (i + 1 < windowSize) { output.push({ time: candles[i].time as UTCTimestamp, value: candles[i].close }); continue }
    let weightSum = 0, alma = 0
    for (let j = 0; j < windowSize; j++) {
      const w = Math.exp(-((j - m) * (j - m)) / (2 * s * s))
      alma += candles[i - windowSize + 1 + j].close * w
      weightSum += w
    }
    output.push({ time: candles[i].time as UTCTimestamp, value: Number((alma / weightSum).toFixed(2)) })
  }
  return output
}

const hiLoSeries = (candles: CandlePoint[]) => {
  if (candles.length === 0) return { hi: [] as { time: UTCTimestamp; value: number }[], lo: [] as { time: UTCTimestamp; value: number }[] }
  let high = -Infinity, low = Infinity
  for (const c of candles) { high = Math.max(high, c.high); low = Math.min(low, c.low) }
  return {
    hi: candles.map((c) => ({ time: c.time as UTCTimestamp, value: Number(high.toFixed(2)) })),
    lo: candles.map((c) => ({ time: c.time as UTCTimestamp, value: Number(low.toFixed(2)) })),
  }
}

const atrTimeSeries = (candles: CandlePoint[], period = 14) => {
  if (candles.length < 2) return []
  const output: { time: UTCTimestamp; value: number }[] = [{ time: candles[0].time as UTCTimestamp, value: candles[0].high - candles[0].low }]
  let prev = candles[0].high - candles[0].low
  for (let i = 1; i < candles.length; i++) {
    const tr = Math.max(candles[i].high - candles[i].low, Math.abs(candles[i].high - candles[i - 1].close), Math.abs(candles[i].low - candles[i - 1].close))
    prev = i < period ? (prev * i + tr) / (i + 1) : (prev * (period - 1) + tr) / period
    output.push({ time: candles[i].time as UTCTimestamp, value: Number(prev.toFixed(2)) })
  }
  return output
}

const stochKSeries = (candles: CandlePoint[], period = 14) =>
  candles.map((_, i) => {
    const slice = candles.slice(Math.max(0, i - period + 1), i + 1)
    const hh = Math.max(...slice.map((c) => c.high))
    const ll = Math.min(...slice.map((c) => c.low))
    return { time: candles[i].time as UTCTimestamp, value: Number((hh === ll ? 50 : ((candles[i].close - ll) / (hh - ll)) * 100).toFixed(2)) }
  })

const adxSeries = (candles: CandlePoint[], period = 14) => {
  if (candles.length < 2) return []
  const output: { time: UTCTimestamp; value: number }[] = [{ time: candles[0].time as UTCTimestamp, value: 25 }]
  const dxValues: number[] = []
  let smoothPDM = 0, smoothNDM = 0, smoothTR = 0, adxSmooth = 25
  for (let i = 1; i < candles.length; i++) {
    const upMove = candles[i].high - candles[i - 1].high
    const downMove = candles[i - 1].low - candles[i].low
    const pdm = upMove > downMove && upMove > 0 ? upMove : 0
    const ndm = downMove > upMove && downMove > 0 ? downMove : 0
    const tr = Math.max(candles[i].high - candles[i].low, Math.abs(candles[i].high - candles[i - 1].close), Math.abs(candles[i].low - candles[i - 1].close))
    if (i <= period) {
      smoothPDM += pdm; smoothNDM += ndm; smoothTR += tr
      if (i === period) { smoothPDM /= period; smoothNDM /= period; smoothTR /= period }
      output.push({ time: candles[i].time as UTCTimestamp, value: 25 }); continue
    }
    smoothPDM = smoothPDM - smoothPDM / period + pdm
    smoothNDM = smoothNDM - smoothNDM / period + ndm
    smoothTR = smoothTR - smoothTR / period + tr
    const pdi = smoothTR === 0 ? 0 : (smoothPDM / smoothTR) * 100
    const ndi = smoothTR === 0 ? 0 : (smoothNDM / smoothTR) * 100
    const dx = pdi + ndi === 0 ? 0 : (Math.abs(pdi - ndi) / (pdi + ndi)) * 100
    dxValues.push(dx)
    adxSmooth = dxValues.length < period ? dx : (adxSmooth * (period - 1) + dx) / period
    output.push({ time: candles[i].time as UTCTimestamp, value: Number(adxSmooth.toFixed(2)) })
  }
  return output
}

const aroonUpTimeSeries = (candles: CandlePoint[], period = 14) =>
  candles.map((_, i) => {
    const slice = candles.slice(Math.max(0, i - period + 1), i + 1)
    let highIdx = 0
    for (let j = 1; j < slice.length; j++) { if (slice[j].high >= slice[highIdx].high) highIdx = j }
    return { time: candles[i].time as UTCTimestamp, value: Number(((highIdx) / Math.max(1, slice.length - 1) * 100).toFixed(2)) }
  })

const accDistSeries = (candles: CandlePoint[]) => {
  let ad = 0
  return candles.map((c) => {
    const mfm = c.high === c.low ? 0 : ((c.close - c.low) - (c.high - c.close)) / (c.high - c.low)
    ad += mfm * Math.max(1, Math.round((c.high - c.low) * 1000))
    return { time: c.time as UTCTimestamp, value: Number(ad.toFixed(2)) }
  })
}

// ─── Smooth brush path (Catmull-Rom → cubic Bézier) ─────────────────────────
const pointsToSmoothPath = (points: OverlayPoint[]): string => {
  if (points.length < 2) return ''
  if (points.length === 2) return `M ${points[0].x} ${points[0].y} L ${points[1].x} ${points[1].y}`
  let d = `M ${points[0].x} ${points[0].y}`
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[Math.max(0, i - 1)]
    const p1 = points[i]
    const p2 = points[i + 1]
    const p3 = points[Math.min(points.length - 1, i + 2)]
    const cp1x = p1.x + (p2.x - p0.x) / 6
    const cp1y = p1.y + (p2.y - p0.y) / 6
    const cp2x = p2.x - (p3.x - p1.x) / 6
    const cp2y = p2.y - (p3.y - p1.y) / 6
    d += ` C ${cp1x} ${cp1y}, ${cp2x} ${cp2y}, ${p2.x} ${p2.y}`
  }
  return d
}

// ─── FIX-1: extendLine rewrite ───────────────────────────────────────────────
// Uses the slab / parametric-ray method correctly.
// t is a real scalar; t=0 is p1, t=1 is p2.  We find all t values where the
// ray crosses the four rectangle edges, then pick the closest exit (forward)
// and, for bothDirs, the closest exit backward.
const extendLine = (
  x1: number, y1: number,
  x2: number, y2: number,
  w: number, h: number,
  bothDirs: boolean,
): [number, number, number, number] => {
  const dx = x2 - x1
  const dy = y2 - y1
  if (dx === 0 && dy === 0) return [x1, y1, x2, y2]

  // t values for intersections with each of the four edges
  const ts: number[] = []
  if (dx !== 0) { ts.push(-x1 / dx); ts.push((w - x1) / dx) }
  if (dy !== 0) { ts.push(-y1 / dy); ts.push((h - y1) / dy) }

  // Forward extent: smallest t > 0 that is still within the rectangle
  let tFwd = Infinity
  for (const t of ts) {
    if (t <= 0) continue
    const px = x1 + dx * t
    const py = y1 + dy * t
    if (px >= -0.5 && px <= w + 0.5 && py >= -0.5 && py <= h + 0.5) {
      if (t < tFwd) tFwd = t
    }
  }
  if (!isFinite(tFwd)) tFwd = 1

  // Backward extent: largest t < 0 still within the rectangle
  let tBwd = -Infinity
  if (bothDirs) {
    for (const t of ts) {
      if (t >= 0) continue
      const px = x1 + dx * t
      const py = y1 + dy * t
      if (px >= -0.5 && px <= w + 0.5 && py >= -0.5 && py <= h + 0.5) {
        if (t > tBwd) tBwd = t
      }
    }
    if (!isFinite(tBwd)) tBwd = 0
  }

  const tStart = bothDirs ? tBwd : 0
  return [
    x1 + dx * tStart, y1 + dy * tStart,
    x1 + dx * tFwd,   y1 + dy * tFwd,
  ]
}

const lowerPaneLabel: Record<Exclude<LowerPaneKey, 'none'>, string> = {
  rsi: 'RSI (14)', macd: 'MACD (12,26,9)', atr: 'ATR (14)',
  stoch: 'Stochastic (14,3)', adx: 'ADX (14)', aroon: 'Aroon Up (14)', accdist: 'Acc/Dist',
}

const lowerPaneColor: Record<Exclude<LowerPaneKey, 'none'>, string> = {
  rsi: '#60A5FA', macd: '#F59E0B', atr: '#A78BFA',
  stoch: '#22D3EE', adx: '#E879F9', aroon: '#34D399', accdist: '#F472B6',
}

const lowerPaneKeyForStudy: Partial<Record<StudyKey, LowerPaneKey>> = {
  rsi14: 'rsi', macd: 'macd', atr14: 'atr',
  stoch14: 'stoch', adx: 'adx', aroonUp: 'aroon', accdist: 'accdist',
}

const toolGroups: { key: ToolKey; icon: ReactNode; tip: string }[][] = [
  [
    { key: 'cursor', icon: <MousePointer2 size={13} />, tip: 'Cursor' },
    { key: 'cross', icon: <Crosshair size={13} />, tip: 'Crosshair' },
  ],
  [
    { key: 'line', icon: <TrendingUp size={13} />, tip: 'Trend Line' },
    { key: 'hline', icon: <Minus size={13} />, tip: 'Horizontal Line' },
    { key: 'vline', icon: <GripVertical size={13} />, tip: 'Vertical Line' },
    { key: 'ray', icon: <Slash size={13} />, tip: 'Ray' },
    { key: 'xline', icon: <MoveHorizontal size={13} />, tip: 'Extended Line' },
    { key: 'arrow', icon: <ArrowUpRight size={13} />, tip: 'Arrow' },
  ],
  [
    { key: 'rect', icon: <Square size={13} />, tip: 'Rectangle' },
    { key: 'fib', icon: <GitCommitHorizontal size={13} />, tip: 'Fib Retracement' },
    { key: 'brush', icon: <PenTool size={13} />, tip: 'Brush' },
  ],
  [
    { key: 'text', icon: <Type size={13} />, tip: 'Text' },
  ],
  [
    { key: 'measure', icon: <Ruler size={13} />, tip: 'Measure' },
    { key: 'zoom', icon: <ZoomIn size={13} />, tip: 'Zoom In' },
    { key: 'eraser', icon: <Eraser size={13} />, tip: 'Eraser' },
  ],
]

type DrawingSnapshot = {
  lines: OverlayLine[]
  rects: OverlayRect[]
  fibs: OverlayFib[]
  labels: OverlayLabel[]
  brushes: OverlayBrush[]
}

export function ChartPanel({ symbol, secondarySymbol, assets, lastPrice, tickDirection, candles1s, candles5s, secondaryCandles1s, secondaryCandles5s, onSelectAsset, onSelectSecondaryAsset, onQuickTrade }: Props) {
  const overlayRef = useRef<SVGSVGElement | null>(null)
  const overlayIdRef = useRef(1)
  const modalDragRef = useRef<{ dragging: boolean; offsetX: number; offsetY: number }>({ dragging: false, offsetX: 0, offsetY: 0 })
  const sectionRef = useRef<HTMLElement | null>(null)
  const undoStackRef = useRef<DrawingSnapshot[]>([])
  const redoStackRef = useRef<DrawingSnapshot[]>([])
  const primaryChartRef = useRef<HTMLDivElement | null>(null)
  const secondaryChartRef = useRef<HTMLDivElement | null>(null)

  const [topHeightPct, setTopHeightPct] = useState(50)
  const splitDragRef = useRef<{ startY: number; startPct: number } | null>(null)

  const onSplitDragStart = useCallback((e: React.PointerEvent) => {
    e.preventDefault()
    splitDragRef.current = { startY: e.clientY, startPct: topHeightPct }
    document.body.style.cursor = 'row-resize'
    document.body.style.userSelect = 'none'
  }, [topHeightPct])

  useEffect(() => {
    const onMove = (e: PointerEvent) => {
      const drag = splitDragRef.current
      if (!drag) return
      const section = sectionRef.current
      if (!section) return
      const rect = section.getBoundingClientRect()
      const deltaY = e.clientY - drag.startY
      const deltaPct = (deltaY / rect.height) * 100
      setTopHeightPct(Math.max(10, Math.min(90, drag.startPct + deltaPct)))
    }
    const onUp = () => {
      splitDragRef.current = null
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
    return () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }
  }, [])

  const [interval, setInterval] = useState<IntervalKey>('1s')
  const [secondaryInterval, setSecondaryInterval] = useState<IntervalKey>('5s')
  const [splitView, setSplitView] = useState(false)
  const [crosshairOn, setCrosshairOn] = useState(true)
  const [activeTool, setActiveTool] = useState<ToolKey>('cursor')
  const [indicatorModalOpen, setIndicatorModalOpen] = useState(false)
  const [indicatorSearch, setIndicatorSearch] = useState('')
  const [indicatorTab, setIndicatorTab] = useState<IndicatorTab>('all')
  const [modalPosition, setModalPosition] = useState<{ x: number; y: number } | null>(null)
  const [favorites, setFavorites] = useState<string[]>(['ema21', 'rsi14'])
  const [lowerPane, setLowerPane] = useState<LowerPaneKey>('none')
  const [, setRedrawTick] = useState(0)
  const [overlayLines, setOverlayLines] = useState<OverlayLine[]>([])
  const [overlayRects, setOverlayRects] = useState<OverlayRect[]>([])
  const [overlayFibs, setOverlayFibs] = useState<OverlayFib[]>([])
  const [overlayLabels, setOverlayLabels] = useState<OverlayLabel[]>([])
  const [overlayBrushes, setOverlayBrushes] = useState<OverlayBrush[]>([])
  const [draftStart, setDraftStart] = useState<OverlayPoint | null>(null)
  const [hoverPoint, setHoverPoint] = useState<OverlayPoint | null>(null)
  const [measureStart, setMeasureStart] = useState<OverlayPoint | null>(null)
  const [measureEnd, setMeasureEnd] = useState<OverlayPoint | null>(null)
  const [showOverlays, setShowOverlays] = useState(true)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [hiddenStudies, setHiddenStudies] = useState<Set<StudyKey>>(new Set())
  const [overlayCollapsed, setOverlayCollapsed] = useState(false)
  const [assetMenuOpen, setAssetMenuOpen] = useState(false)
  const [assetSearch, setAssetSearch] = useState('')
  const [quickTrade, setQuickTrade] = useState<{
    action: 'buy' | 'sell'
    quantity: string
    orderType: 'limit' | 'market'
    limitPrice: string
    stopLoss: string
    target: string
    useRisk: boolean
  } | null>(null)
  const [quickTradePos, setQuickTradePos] = useState({ x: 28, y: 44 })
  const quickDragRef = useRef<{ dragging: boolean; dx: number; dy: number }>({ dragging: false, dx: 0, dy: 0 })
  const [studies, setStudies] = useState<Record<StudyKey, boolean>>({
    ema9: true, ema21: true, ema50: false, ema200: false,
    sma20: false, sma50: false, wma20: false, vwap: false,
    bbUpper: false, bbMid: false, bbLower: false,
    rsi14: true, macd: false, atr14: false, stoch14: false, supertrend: false,
    avgprice: false, alma: false, adx: false, aroonUp: false, accdist: false,
    hi52: false, lo52: false,
  })

  // ─── FIX-2: keep clip rect in sync with the SVG container size ──────────
  useEffect(() => {
    const svg = overlayRef.current
    if (!svg) return
    const ro = new ResizeObserver((entries) => {
      const box = entries[0]?.contentRect
      if (!box) return
      setClipW(Math.max(0, box.width - 56)) // PRICE_SCALE_WIDTH
      setClipH(Math.max(0, box.height - 24)) // TIME_SCALE_HEIGHT
    })
    ro.observe(svg)
    // Initial read
    const box = svg.getBoundingClientRect()
    setClipW(Math.max(0, box.width - 56))
    setClipH(Math.max(0, box.height - 24))
    return () => ro.disconnect()
  }, [])

  const candleMap = useMemo(() => ({
    '1s': candles1s,
    '5s': candles5s,
    '15s': aggregateCandles(candles1s, timeframeToSeconds['15s']),
    '30s': aggregateCandles(candles1s, timeframeToSeconds['30s']),
    '1m': aggregateCandles(candles1s, timeframeToSeconds['1m']),
    '3m': aggregateCandles(candles1s, timeframeToSeconds['3m']),
    '5m': aggregateCandles(candles1s, timeframeToSeconds['5m']),
    '15m': aggregateCandles(candles1s, timeframeToSeconds['15m']),
  } as Record<IntervalKey, CandlePoint[]>), [candles1s, candles5s])

  const activeCandles = useMemo(() => candleMap[interval], [candleMap, interval])

  const secondaryCandleMap = useMemo(() => {
    if (!secondaryCandles1s || !secondaryCandles5s) return candleMap
    return {
      '1s': secondaryCandles1s,
      '5s': secondaryCandles5s,
      '15s': aggregateCandles(secondaryCandles1s, 15),
      '30s': aggregateCandles(secondaryCandles1s, 30),
      '1m': aggregateCandles(secondaryCandles1s, 60),
      '3m': aggregateCandles(secondaryCandles1s, 180),
      '5m': aggregateCandles(secondaryCandles1s, 300),
      '15m': aggregateCandles(secondaryCandles1s, 900),
    } as Record<IntervalKey, CandlePoint[]>
  }, [secondaryCandles1s, secondaryCandles5s, candleMap])

  const secondaryCandles = useMemo(() => secondaryCandleMap[secondaryInterval], [secondaryInterval, secondaryCandleMap])

  const stableActiveCandles = useMemo(() => (activeCandles.length > 1 ? activeCandles.slice(0, -1) : activeCandles), [activeCandles])
  const stableSecondaryCandles = useMemo(() => (secondaryCandles.length > 1 ? secondaryCandles.slice(0, -1) : secondaryCandles), [secondaryCandles])

  const [secAssetMenuOpen, setSecAssetMenuOpen] = useState(false)
  const [secAssetSearch, setSecAssetSearch] = useState('')
  const filteredSecAssets = useMemo(() => {
    const q = secAssetSearch.trim().toLowerCase()
    return q ? assets.filter((a) => a.toLowerCase().includes(q)).slice(0, 12) : assets.slice(0, 12)
  }, [assets, secAssetSearch])

  // Wheel handling correctly wrapped down here inside the component
  const lowerPaneChartRef = useRef<HTMLDivElement | null>(null)
  const apiRef = useRef<IChartApi | null>(null)
  const seriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const secondaryApiRef = useRef<IChartApi | null>(null)
  const secondarySeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const lowerApiRef = useRef<IChartApi | null>(null)
  const lowerSeriesRef = useRef<ISeriesApi<'Line'> | null>(null)
  const primaryStudiesRef = useRef<StudySeriesMap>({})
  const secondaryStudiesRef = useRef<StudySeriesMap>({})
  const seededRef = useRef<string | null>(null)
  const secondarySeededRef = useRef<string | null>(null)
  const lastCandleRef = useRef<number>(0)
  const lastSecondaryCandleRef = useRef<number>(0)

  useEffect(() => {
    const el = primaryChartRef.current;
    if (!el) return;
    const handleWheel = (e: WheelEvent) => { if (e.ctrlKey) e.preventDefault(); };
    el.addEventListener("wheel", handleWheel, { passive: false });
    return () => el.removeEventListener("wheel", handleWheel);
  }, []);

  useEffect(() => {
    const el = secondaryChartRef.current;
    if (!el) return;
    const handleWheel = (e: WheelEvent) => { if (e.ctrlKey) e.preventDefault(); };
    el.addEventListener("wheel", handleWheel, { passive: false });
    return () => el.removeEventListener("wheel", handleWheel);
  }, []);

  const isBrushingRef = useRef(false)
  const activeBrushIdRef = useRef<number | null>(null)
  const liveBrushPointsRef = useRef<OverlayPoint[]>([])
  const brushRafRef = useRef<number | null>(null)

  const [clipW, setClipW] = useState(0)
  const [clipH, setClipH] = useState(0)

  const metrics = useMemo(() => ({
    rsi14: studies.rsi14 ? rsi(activeCandles) : 50,
    macd: studies.macd ? macd(activeCandles) : { line: 0, signal: 0, hist: 0 },
    atr14: studies.atr14 ? atr(activeCandles) : 0,
    stoch14: studies.stoch14 ? stoch(activeCandles) : { k: 0, d: 0 },
    supertrend: studies.supertrend ? supertrend(activeCandles) : { value: 0, direction: 'UP' as const },
    adx: studies.adx ? (() => { const d = adxSeries(activeCandles); return d.length ? d[d.length - 1].value : 25 })() : 25,
  }), [activeCandles, studies.rsi14, studies.macd, studies.atr14, studies.stoch14, studies.supertrend, studies.adx])

  const latestCandle = useMemo(() => activeCandles[activeCandles.length - 1], [activeCandles])
  const previousClose = useMemo(() => {
    if (activeCandles.length < 2) return latestCandle?.close ?? lastPrice
    return activeCandles[activeCandles.length - 2].close
  }, [activeCandles, latestCandle, lastPrice])
  const delta = (latestCandle?.close ?? lastPrice) - previousClose
  const deltaPct = previousClose === 0 ? 0 : (delta / previousClose) * 100

  const filteredIndicators = useMemo(() => {
    const query = indicatorSearch.trim().toLowerCase()
    const tabSource =
      indicatorTab === 'favorites' ? indicatorCatalog.filter((i) => favorites.includes(i.id))
      : indicatorTab === 'builtins' ? indicatorCatalog.filter((i) => Boolean(i.key))
      : indicatorCatalog
    const source = query.length ? tabSource.filter((i) => i.label.toLowerCase().includes(query) || i.category.toLowerCase().includes(query)) : tabSource
    return [...source].sort((a, b) => Number(favorites.includes(b.id)) - Number(favorites.includes(a.id)))
  }, [indicatorSearch, indicatorTab, favorites])

  const filteredAssets = useMemo(() => {
    const q = assetSearch.trim().toLowerCase()
    return q ? assets.filter((a) => a.toLowerCase().includes(q)).slice(0, 12) : assets.slice(0, 12)
  }, [assets, assetSearch])

  const activeStudyChips = useMemo(() => {
    const candles = stableActiveCandles
    if (!candles.length) return []
    const enabled = overlayStudies.filter((s) => studies[s.key])
    if (!enabled.length) return []
    let bbData: ReturnType<typeof bbands> | null = null
    let hlData: ReturnType<typeof hiLoSeries> | null = null
    const last = (arr: { value: number }[]) => arr.length ? arr[arr.length - 1].value.toFixed(2) : '--'
    const valueFor = (key: StudyKey): string => {
      switch (key) {
        case 'ema9': return last(ema(candles, 9))
        case 'ema21': return last(ema(candles, 21))
        case 'ema50': return last(ema(candles, 50))
        case 'ema200': return last(ema(candles, 200))
        case 'sma20': return last(sma(candles, 20))
        case 'sma50': return last(sma(candles, 50))
        case 'wma20': return last(wma(candles, 20))
        case 'vwap': return last(vwap(candles))
        case 'bbUpper': return last((bbData ??= bbands(candles)).upper)
        case 'bbMid': return last((bbData ??= bbands(candles)).mid)
        case 'bbLower': return last((bbData ??= bbands(candles)).lower)
        case 'supertrend': return last(supertrendSeries(candles))
        case 'avgprice': return last(avgPriceSeries(candles))
        case 'alma': return last(almaSeries(candles))
        case 'hi52': return last((hlData ??= hiLoSeries(candles)).hi)
        case 'lo52': return last((hlData ??= hiLoSeries(candles)).lo)
        default: return ''
      }
    }
    return enabled.map((s) => ({ key: s.key, label: s.label, value: valueFor(s.key) }))
  }, [studies, stableActiveCandles])

  const secondaryActiveStudyChips = useMemo(() => {
    const candles = stableSecondaryCandles
    if (!candles.length) return []
    const enabled = overlayStudies.filter((s) => studies[s.key])
    if (!enabled.length) return []
    let bbData: ReturnType<typeof bbands> | null = null
    let hlData: ReturnType<typeof hiLoSeries> | null = null
    const last = (arr: { value: number }[]) => arr.length ? arr[arr.length - 1].value.toFixed(2) : '--'
    const valueFor = (key: StudyKey): string => {
      switch (key) {
        case 'ema9': return last(ema(candles, 9))
        case 'ema21': return last(ema(candles, 21))
        case 'ema50': return last(ema(candles, 50))
        case 'ema200': return last(ema(candles, 200))
        case 'sma20': return last(sma(candles, 20))
        case 'sma50': return last(sma(candles, 50))
        case 'wma20': return last(wma(candles, 20))
        case 'vwap': return last(vwap(candles))
        case 'bbUpper': return last((bbData ??= bbands(candles)).upper)
        case 'bbMid': return last((bbData ??= bbands(candles)).mid)
        case 'bbLower': return last((bbData ??= bbands(candles)).lower)
        case 'supertrend': return last(supertrendSeries(candles))
        case 'avgprice': return last(avgPriceSeries(candles))
        case 'alma': return last(almaSeries(candles))
        case 'hi52': return last((hlData ??= hiLoSeries(candles)).hi)
        case 'lo52': return last((hlData ??= hiLoSeries(candles)).lo)
        default: return ''
      }
    }
    return enabled.map((s) => ({ key: s.key, label: s.label, value: valueFor(s.key) }))
  }, [studies, stableSecondaryCandles])

  const drawingTools: ToolKey[] = ['line', 'hline', 'vline', 'ray', 'xline', 'arrow', 'rect', 'fib', 'brush', 'text', 'measure', 'eraser']
  const overlayInteractive = drawingTools.includes(activeTool)

  // ─── FIX-2: clamp helper — keeps points within the plot area ────────────
  const clampToPlot = (p: OverlayPoint): OverlayPoint => ({
    x: Math.max(0, Math.min(clipW > 0 ? clipW : (overlayRef.current?.clientWidth ?? 800) - PRICE_SCALE_WIDTH, p.x)),
    y: Math.max(0, Math.min(clipH > 0 ? clipH : (overlayRef.current?.clientHeight ?? 500) - TIME_SCALE_HEIGHT, p.y)),
  })

  const getOverlayPoint = (event: ReactMouseEvent<SVGSVGElement>): OverlayPoint | null => {
    if (!overlayRef.current) return null
    const rect = overlayRef.current.getBoundingClientRect()
    const raw = clampToPlot({ x: event.clientX - rect.left, y: event.clientY - rect.top })
    const logical = apiRef.current?.timeScale().coordinateToLogical(raw.x) ?? undefined
    const price = seriesRef.current?.coordinateToPrice(raw.y) ?? undefined
    return { ...raw, logical, price }
  }

  const nextOverlayId = () => { const id = overlayIdRef.current; overlayIdRef.current += 1; return id }

  const handleOverlayClick = (event: ReactMouseEvent<SVGSVGElement>) => {
    event.stopPropagation()
    event.preventDefault()
    const point = getOverlayPoint(event)
    if (!point) return
    const svgW = clipW || (overlayRef.current?.clientWidth ?? 800) - PRICE_SCALE_WIDTH
    const svgH = clipH || (overlayRef.current?.clientHeight ?? 500) - TIME_SCALE_HEIGHT

    if (activeTool === 'line' || activeTool === 'ray' || activeTool === 'xline' || activeTool === 'arrow') {
      if (!draftStart) { setDraftStart(point); return }
      pushUndoSnapshot()
      setOverlayLines((prev) => [...prev, { id: nextOverlayId(), kind: activeTool as LineKind, x1: draftStart.x, y1: draftStart.y, logical1: draftStart.logical, price1: draftStart.price, x2: point.x, y2: point.y, logical2: point.logical, price2: point.price }])
      setDraftStart(null); setHoverPoint(null); return
    }
    if (activeTool === 'hline') {
      pushUndoSnapshot()
      setOverlayLines((prev) => [...prev, { id: nextOverlayId(), kind: 'hline', x1: 0, y1: point.y, x2: svgW, y2: point.y }]); return
    }
    if (activeTool === 'vline') {
      pushUndoSnapshot()
      setOverlayLines((prev) => [...prev, { id: nextOverlayId(), kind: 'vline', x1: point.x, y1: 0, x2: point.x, y2: svgH }]); return
    }
    if (activeTool === 'rect' || activeTool === 'fib') {
      if (!draftStart) { setDraftStart(point); return }
      pushUndoSnapshot()
      if (activeTool === 'rect') setOverlayRects((prev) => [...prev, { id: nextOverlayId(), x1: draftStart.x, y1: draftStart.y, logical1: draftStart.logical, price1: draftStart.price, x2: point.x, y2: point.y, logical2: point.logical, price2: point.price }])
      else setOverlayFibs((prev) => [...prev, { id: nextOverlayId(), x1: draftStart.x, y1: draftStart.y, logical1: draftStart.logical, price1: draftStart.price, x2: point.x, y2: point.y, logical2: point.logical, price2: point.price }])
      setDraftStart(null); setHoverPoint(null); return
    }
    if (activeTool === 'text') {
      const text = window.prompt('Label text', 'Note')?.trim()
      if (!text) return
      pushUndoSnapshot()
      setOverlayLabels((prev) => [...prev, { id: nextOverlayId(), x: point.x, y: point.y, logical: point.logical, price: point.price, text }]); return
    }
    if (activeTool === 'eraser') {
      clearToolDrafts(); return
    }
    if (activeTool === 'measure') {
      if (!measureStart || (measureStart && measureEnd)) { setMeasureStart(point); setMeasureEnd(null) }
      else setMeasureEnd(point)
    }
  }

  // ─── FIX-3: rAF-throttled mousemove ─────────────────────────────────────
  const handleOverlayMove = (event: ReactMouseEvent<SVGSVGElement>) => {
    const point = getOverlayPoint(event)
    if (!point) return

    // Only update hoverPoint when we actually need the preview line/rect
    if (draftStart || (activeTool === 'measure' && measureStart && !measureEnd)) {
      setHoverPoint(point)
    }

    if (activeTool === 'brush' && isBrushingRef.current) {
      liveBrushPointsRef.current.push(point)
      // Schedule at most one state update per animation frame
      if (brushRafRef.current === null) {
        brushRafRef.current = requestAnimationFrame(() => {
          brushRafRef.current = null
          pointsToSmoothPath(liveBrushPointsRef.current)
        })
      }
    }
  }

  const handleOverlayMouseDown = (event: ReactMouseEvent<SVGSVGElement>) => {
    if (activeTool !== 'brush') return
    const point = getOverlayPoint(event)
    if (!point) return
    pushUndoSnapshot()
    const id = nextOverlayId()
    activeBrushIdRef.current = id
    isBrushingRef.current = true
    liveBrushPointsRef.current = [point]
  }

  const handleOverlayMouseUp = () => {
    if (activeTool === 'brush' && isBrushingRef.current) {
      // Cancel any pending rAF to avoid stale update after commit
      if (brushRafRef.current !== null) { cancelAnimationFrame(brushRafRef.current); brushRafRef.current = null }
      const points = liveBrushPointsRef.current
      if (points.length >= 2) {
        const id = activeBrushIdRef.current ?? nextOverlayId()
        setOverlayBrushes((prev) => [...prev, { id, points }])
      }
      isBrushingRef.current = false
      activeBrushIdRef.current = null
      liveBrushPointsRef.current = []
    }
  }

  const clearToolDrafts = () => {
    setDraftStart(null); setHoverPoint(null); setMeasureStart(null); setMeasureEnd(null)
  }

  const pushUndoSnapshot = () => {
    undoStackRef.current.push({ lines: [...overlayLines], rects: [...overlayRects], fibs: [...overlayFibs], labels: [...overlayLabels], brushes: [...overlayBrushes] })
    redoStackRef.current = []
  }

  const handleUndo = useCallback(() => {
    const snap = undoStackRef.current.pop()
    if (!snap) return
    setOverlayLines((prev) => {
      redoStackRef.current.push({ lines: [...prev], rects: [...overlayRects], fibs: [...overlayFibs], labels: [...overlayLabels], brushes: [...overlayBrushes] })
      return snap.lines
    })
    setOverlayRects(snap.rects); setOverlayFibs(snap.fibs); setOverlayLabels(snap.labels); setOverlayBrushes(snap.brushes)
  }, [overlayRects, overlayFibs, overlayLabels, overlayBrushes])

  const handleRedo = useCallback(() => {
    const snap = redoStackRef.current.pop()
    if (!snap) return
    setOverlayLines((prev) => {
      undoStackRef.current.push({ lines: [...prev], rects: [...overlayRects], fibs: [...overlayFibs], labels: [...overlayLabels], brushes: [...overlayBrushes] })
      return snap.lines
    })
    setOverlayRects(snap.rects); setOverlayFibs(snap.fibs); setOverlayLabels(snap.labels); setOverlayBrushes(snap.brushes)
  }, [overlayRects, overlayFibs, overlayLabels, overlayBrushes])

  const handleScreenshot = () => {
    const canvas = primaryChartRef.current?.querySelector('canvas')
    if (!canvas) return
    const link = document.createElement('a')
    link.download = `chart-${Date.now()}.png`
    link.href = canvas.toDataURL('image/png')
    link.click()
  }

  const toggleFullscreen = () => {
    if (!sectionRef.current) return
    if (document.fullscreenElement) { document.exitFullscreen(); setIsFullscreen(false) }
    else { sectionRef.current.requestFullscreen(); setIsFullscreen(true) }
  }

  const openIndicatorModal = () => {
    if (!modalPosition) {
      const x = Math.max(20, (window.innerWidth - 430) / 2)
      const y = Math.max(24, window.innerHeight * 0.12)
      setModalPosition({ x, y })
    }
    setIndicatorModalOpen(true)
  }

  const openQuickTrade = (action: 'buy' | 'sell') =>
    setQuickTrade({ action, quantity: '0.010', orderType: 'market', limitPrice: lastPrice.toFixed(2), stopLoss: '', target: '', useRisk: false })

  const submitQuickTrade = () => {
    if (!quickTrade || !onQuickTrade) return
    const quantity = Number(quickTrade.quantity)
    const limitPrice = Number(quickTrade.limitPrice)
    if (!Number.isFinite(quantity) || quantity <= 0) return
    if (quickTrade.orderType === 'limit' && (!Number.isFinite(limitPrice) || limitPrice <= 0)) return
    if (quickTrade.useRisk) {
      const sl = Number(quickTrade.stopLoss), tg = Number(quickTrade.target)
      const ref = quickTrade.orderType === 'market' ? lastPrice : limitPrice
      if (!Number.isFinite(sl) || sl <= 0 || !Number.isFinite(tg) || tg <= 0) return
      if (quickTrade.action === 'buy' && !(sl < ref && tg > ref)) return
      if (quickTrade.action === 'sell' && !(sl > ref && tg < ref)) return
    }
    onQuickTrade(quickTrade.action, quantity, quickTrade.orderType, quickTrade.orderType === 'limit' ? limitPrice : lastPrice)
    setQuickTrade(null)
  }

  useEffect(() => {
    if (!quickTrade || quickTrade.orderType !== 'market') return
    const live = lastPrice.toFixed(2)
    if (quickTrade.limitPrice === live) return
    setQuickTrade((prev) => (prev && prev.orderType === 'market' ? { ...prev, limitPrice: live } : prev))
  }, [lastPrice, quickTrade])

  const quickTradeError = useMemo(() => {
    if (!quickTrade) return ''
    const quantity = Number(quickTrade.quantity)
    if (!Number.isFinite(quantity) || quantity <= 0) return 'Enter valid quantity.'
    const limitPrice = Number(quickTrade.limitPrice)
    if (quickTrade.orderType === 'limit' && (!Number.isFinite(limitPrice) || limitPrice <= 0)) return 'Enter valid limit price.'
    if (!quickTrade.useRisk) return ''
    const sl = Number(quickTrade.stopLoss), tg = Number(quickTrade.target)
    const ref = quickTrade.orderType === 'market' ? lastPrice : limitPrice
    if (!Number.isFinite(sl) || sl <= 0) return 'Enter valid stoploss.'
    if (!Number.isFinite(tg) || tg <= 0) return 'Enter valid target.'
    if (quickTrade.action === 'buy') {
      if (sl >= ref) return 'For BUY, stoploss must be below current price.'
      if (tg <= ref) return 'For BUY, target must be above current price.'
    } else {
      if (sl <= ref) return 'For SELL, stoploss must be above current price.'
      if (tg >= ref) return 'For SELL, target must be below current price.'
    }
    return ''
  }, [quickTrade, lastPrice])

  const twoClickTools: ToolKey[] = ['line', 'brush', 'ray', 'xline', 'arrow', 'rect', 'fib', 'measure']
  const selectTool = (tool: ToolKey) => { setActiveTool(tool); if (!twoClickTools.includes(tool)) clearToolDrafts() }

  const startModalDrag = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (!modalPosition) return
    modalDragRef.current.dragging = true
    modalDragRef.current.offsetX = event.clientX - modalPosition.x
    modalDragRef.current.offsetY = event.clientY - modalPosition.y
  }

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!modalDragRef.current.dragging) return
      setModalPosition({ x: Math.max(12, e.clientX - modalDragRef.current.offsetX), y: Math.max(12, e.clientY - modalDragRef.current.offsetY) })
    }
    const onUp = () => { modalDragRef.current.dragging = false }
    window.addEventListener('mousemove', onMove); window.addEventListener('mouseup', onUp)
    return () => { window.removeEventListener('mousemove', onMove); window.removeEventListener('mouseup', onUp) }
  }, [])

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!quickDragRef.current.dragging) return
      setQuickTradePos({ x: Math.max(10, e.clientX - quickDragRef.current.dx), y: Math.max(28, e.clientY - quickDragRef.current.dy) })
    }
    const onUp = () => { quickDragRef.current.dragging = false }
    window.addEventListener('mousemove', onMove); window.addEventListener('mouseup', onUp)
    return () => { window.removeEventListener('mousemove', onMove); window.removeEventListener('mouseup', onUp) }
  }, [])

  // Global mouseup — commits brush stroke even if released outside SVG
  useEffect(() => {
    const onUp = () => {
      if (!isBrushingRef.current) return
      if (brushRafRef.current !== null) { cancelAnimationFrame(brushRafRef.current); brushRafRef.current = null }
      const points = liveBrushPointsRef.current
      if (points.length >= 2) {
        const id = activeBrushIdRef.current ?? overlayIdRef.current++
        setOverlayBrushes((prev) => [...prev, { id, points }])
      }
      isBrushingRef.current = false
      activeBrushIdRef.current = null
      liveBrushPointsRef.current = []
    }
    window.addEventListener('mouseup', onUp)
    return () => window.removeEventListener('mouseup', onUp)
  }, [])

  useEffect(() => {
    const onFsChange = () => setIsFullscreen(Boolean(document.fullscreenElement))
    document.addEventListener('fullscreenchange', onFsChange)
    return () => document.removeEventListener('fullscreenchange', onFsChange)
  }, [])

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (indicatorModalOpen && e.key === 'Escape') { setIndicatorModalOpen(false); return }
      if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) { e.preventDefault(); handleUndo() }
      if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || (e.key === 'z' && e.shiftKey))) { e.preventDefault(); handleRedo() }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [indicatorModalOpen, handleUndo, handleRedo])

  useEffect(() => {
    const drawing = drawingTools.includes(activeTool)
    const scrollEnabled = !drawing && (activeTool === 'move' || activeTool === 'cross' || activeTool === 'cursor')
    const scaleEnabled = !drawing
    const opts = {
      handleScroll: scrollEnabled,
      handleScale: scaleEnabled ? { mouseWheel: true, pinch: true, axisPressedMouseMove: true } : false as const,
      crosshair: { mode: crosshairOn ? (activeTool === 'magnet' ? 1 : 0) : 1 },
    }
    apiRef.current?.applyOptions(opts)
    secondaryApiRef.current?.applyOptions(opts)
  }, [activeTool, crosshairOn])

  const applyStudyData = useCallback(
    (candles: CandlePoint[], map: StudySeriesMap) => {
      let bbData: ReturnType<typeof bbands> | null = null
      let hlData: ReturnType<typeof hiLoSeries> | null = null
      const compute = (key: StudyKey): { time: UTCTimestamp; value: number }[] => {
        switch (key) {
          case 'ema9': return ema(candles, 9)
          case 'ema21': return ema(candles, 21)
          case 'ema50': return ema(candles, 50)
          case 'ema200': return ema(candles, 200)
          case 'sma20': return sma(candles, 20)
          case 'sma50': return sma(candles, 50)
          case 'wma20': return wma(candles, 20)
          case 'vwap': return vwap(candles)
          case 'bbUpper': return (bbData ??= bbands(candles)).upper
          case 'bbMid': return (bbData ??= bbands(candles)).mid
          case 'bbLower': return (bbData ??= bbands(candles)).lower
          case 'supertrend': return supertrendSeries(candles)
          case 'avgprice': return avgPriceSeries(candles)
          case 'alma': return almaSeries(candles)
          case 'hi52': return (hlData ??= hiLoSeries(candles)).hi
          case 'lo52': return (hlData ??= hiLoSeries(candles)).lo
          default: return []
        }
      }
      overlayStudies.forEach((study) => {
        const series = map[study.key]
        if (!series) return
        if (!studies[study.key] || hiddenStudies.has(study.key)) { series.setData([]); return }
        series.setData(compute(study.key))
      })
    },
    [studies, hiddenStudies],
  )

  useEffect(() => {
    if (!primaryChartRef.current) return
    const chart = createChart(primaryChartRef.current, {
      layout: { background: { color: '#0B0E11' }, textColor: '#AAB0B6' },
      rightPriceScale: { borderColor: '#2B2F36' },
      timeScale: { borderColor: '#2B2F36', timeVisible: true, secondsVisible: true },
      grid: { horzLines: { color: '#1C2128' }, vertLines: { color: '#1C2128' } },
      crosshair: { mode: crosshairOn ? 0 : 1 },
      width: primaryChartRef.current.clientWidth,
      height: primaryChartRef.current.clientHeight,
    })
    seriesRef.current = chart.addSeries(CandlestickSeries, {
      upColor: '#00C076', downColor: '#FF3B30',
      borderUpColor: '#00C076', borderDownColor: '#FF3B30',
      wickUpColor: '#00C076', wickDownColor: '#FF3B30',
    })
    apiRef.current = chart
    const onRangeChange = () => setRedrawTick((t) => t + 1)
    chart.timeScale().subscribeVisibleLogicalRangeChange(onRangeChange)
    overlayStudies.forEach((study) => {
      primaryStudiesRef.current[study.key] = chart.addSeries(LineSeries, {
        color: study.color, lineWidth: study.key === 'bbMid' ? 1 : 2, lastValueVisible: false, priceLineVisible: false,
      })
    })
    const observer = new ResizeObserver((entries) => {
      const box = entries[0]?.contentRect
      if (box) chart.applyOptions({ width: box.width, height: box.height })
    })
    observer.observe(primaryChartRef.current)
    return () => { chart.timeScale().unsubscribeVisibleLogicalRangeChange(onRangeChange); observer.disconnect(); chart.remove(); apiRef.current = null; seriesRef.current = null; primaryStudiesRef.current = {}; seededRef.current = null }
  }, [crosshairOn])

  useEffect(() => {
    if (!splitView || !secondaryChartRef.current) {
      secondaryApiRef.current?.remove(); secondaryApiRef.current = null; secondarySeriesRef.current = null; secondaryStudiesRef.current = {}; secondarySeededRef.current = null; return
    }
    const chart = createChart(secondaryChartRef.current, {
      layout: { background: { color: '#0B0E11' }, textColor: '#AAB0B6' },
      rightPriceScale: { borderColor: '#2B2F36' },
      timeScale: { borderColor: '#2B2F36', timeVisible: true, secondsVisible: true },
      grid: { horzLines: { color: '#1C2128' }, vertLines: { color: '#1C2128' } },
      crosshair: { mode: crosshairOn ? 0 : 1 },
      width: secondaryChartRef.current.clientWidth,
      height: secondaryChartRef.current.clientHeight,
    })
    secondarySeriesRef.current = chart.addSeries(CandlestickSeries, {
      upColor: '#00C076', downColor: '#FF3B30',
      borderUpColor: '#00C076', borderDownColor: '#FF3B30',
      wickUpColor: '#00C076', wickDownColor: '#FF3B30',
    })
    secondaryApiRef.current = chart
    const onRangeChange = () => setRedrawTick((t) => t + 1)
    chart.timeScale().subscribeVisibleLogicalRangeChange(onRangeChange)
    overlayStudies.forEach((study) => {
      secondaryStudiesRef.current[study.key] = chart.addSeries(LineSeries, {
        color: study.color, lineWidth: study.key === 'bbMid' ? 1 : 2, lastValueVisible: false, priceLineVisible: false,
      })
    })
    const observer = new ResizeObserver((entries) => {
      const box = entries[0]?.contentRect
      if (box) chart.applyOptions({ width: box.width, height: box.height })
    })
    observer.observe(secondaryChartRef.current)
    return () => { chart.timeScale().unsubscribeVisibleLogicalRangeChange(onRangeChange); observer.disconnect(); chart.remove(); secondaryApiRef.current = null; secondarySeriesRef.current = null; secondaryStudiesRef.current = {}; secondarySeededRef.current = null }
  }, [splitView, crosshairOn])

  useEffect(() => {
    const series = seriesRef.current
    if (!series || !activeCandles.length) return
    const latest = activeCandles[activeCandles.length - 1]
    const seedKey = `${symbol}:${interval}`
    if (seededRef.current !== seedKey) {
      series.setData(toCandleSeries(activeCandles))
      applyStudyData(stableActiveCandles, primaryStudiesRef.current)
      apiRef.current?.timeScale().fitContent()
      seededRef.current = seedKey; lastCandleRef.current = latest.time; return
    }
    series.update({ time: latest.time as UTCTimestamp, open: latest.open, high: latest.high, low: latest.low, close: latest.close })
    if (latest.time !== lastCandleRef.current) {
      applyStudyData(stableActiveCandles, primaryStudiesRef.current)
      apiRef.current?.timeScale().scrollToRealTime()
      lastCandleRef.current = latest.time
    }
  }, [activeCandles, stableActiveCandles, interval, symbol, applyStudyData])

  useEffect(() => {
    if (!splitView) return
    const series = secondarySeriesRef.current
    if (!series || !secondaryCandles.length) return
    const latest = secondaryCandles[secondaryCandles.length - 1]
    const seedKey = `${secondarySymbol ?? symbol}:${secondaryInterval}`
    if (secondarySeededRef.current !== seedKey) {
      series.setData(toCandleSeries(secondaryCandles))
      applyStudyData(stableSecondaryCandles, secondaryStudiesRef.current)
      secondaryApiRef.current?.timeScale().fitContent()
      secondarySeededRef.current = seedKey; lastSecondaryCandleRef.current = latest.time; return
    }
    series.update({ time: latest.time as UTCTimestamp, open: latest.open, high: latest.high, low: latest.low, close: latest.close })
    if (latest.time !== lastSecondaryCandleRef.current) {
      applyStudyData(stableSecondaryCandles, secondaryStudiesRef.current)
      secondaryApiRef.current?.timeScale().scrollToRealTime()
      lastSecondaryCandleRef.current = latest.time
    }
  }, [secondaryCandles, stableSecondaryCandles, secondaryInterval, splitView, secondarySymbol, symbol, applyStudyData])

  useEffect(() => {
    if (seededRef.current === `${symbol}:${interval}`) applyStudyData(stableActiveCandles, primaryStudiesRef.current)
    if (splitView && secondarySeededRef.current === `${secondarySymbol ?? symbol}:${secondaryInterval}`) applyStudyData(stableSecondaryCandles, secondaryStudiesRef.current)
  }, [applyStudyData, interval, splitView, secondaryInterval, stableActiveCandles, stableSecondaryCandles, symbol, secondarySymbol])

  useEffect(() => {
    if (lowerPane === 'none' || !lowerPaneChartRef.current) {
      lowerApiRef.current?.remove(); lowerApiRef.current = null; lowerSeriesRef.current = null; return
    }
    const chart = createChart(lowerPaneChartRef.current, {
      layout: { background: { color: '#0B0E11' }, textColor: '#AAB0B6' },
      rightPriceScale: { borderColor: '#2B2F36' },
      timeScale: { borderColor: '#2B2F36', timeVisible: true, secondsVisible: true },
      grid: { horzLines: { color: '#1C2128' }, vertLines: { color: '#1C2128' } },
      width: lowerPaneChartRef.current.clientWidth,
      height: lowerPaneChartRef.current.clientHeight,
    })
    const series = chart.addSeries(LineSeries, { color: lowerPaneColor[lowerPane], lineWidth: 2, lastValueVisible: false, priceLineVisible: false })
    lowerApiRef.current = chart; lowerSeriesRef.current = series
    const observer = new ResizeObserver((entries) => {
      const box = entries[0]?.contentRect
      if (box) chart.applyOptions({ width: box.width, height: box.height })
    })
    observer.observe(lowerPaneChartRef.current)
    return () => { observer.disconnect(); chart.remove(); lowerApiRef.current = null; lowerSeriesRef.current = null }
  }, [lowerPane])

  useEffect(() => {
    if (lowerPane === 'none' || !lowerSeriesRef.current || !activeCandles.length) return
    const dataMap: Record<Exclude<LowerPaneKey, 'none'>, () => { time: UTCTimestamp; value: number }[]> = {
      rsi: () => rsiSeries(activeCandles), macd: () => macdSeries(activeCandles),
      atr: () => atrTimeSeries(activeCandles), stoch: () => stochKSeries(activeCandles),
      adx: () => adxSeries(activeCandles), aroon: () => aroonUpTimeSeries(activeCandles),
      accdist: () => accDistSeries(activeCandles),
    }
    lowerSeriesRef.current.setData(dataMap[lowerPane]())
    lowerApiRef.current?.timeScale().fitContent()
  }, [lowerPane, activeCandles])

  const resetView = () => { apiRef.current?.timeScale().fitContent(); secondaryApiRef.current?.timeScale().fitContent() }
  const toggleFavorite = (id: string) => setFavorites((prev) => prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id])

  const toggleStudy = (item: IndicatorItem) => {
    if (!item.key) return
    if (item.id === 'bbands') { setStudies((prev) => ({ ...prev, bbUpper: !prev.bbUpper, bbMid: !prev.bbMid, bbLower: !prev.bbLower })); return }
    if (item.id === '52w') { setStudies((prev) => ({ ...prev, hi52: !prev.hi52, lo52: !prev.lo52 })); return }
    const key = item.key as StudyKey
    const enabledNext = !studies[key]
    setStudies((prev) => ({ ...prev, [key]: enabledNext }))
    const paneKey = lowerPaneKeyForStudy[key]
    if (paneKey) setLowerPane(enabledNext ? paneKey : lowerPane === paneKey ? 'none' : lowerPane)
  }

  const isStudyEnabled = (item: IndicatorItem) => {
    if (!item.key) return false
    if (item.id === 'bbands') return studies.bbUpper || studies.bbMid || studies.bbLower
    if (item.id === '52w') return studies.hi52 || studies.lo52
    return studies[item.key]
  }

  const applyPreset = (preset: 'scalp' | 'swing' | 'trend') => {
    const next: Record<StudyKey, boolean> = {
      ema9: false, ema21: false, ema50: false, ema200: false, sma20: false, sma50: false, wma20: false, vwap: false,
      bbUpper: false, bbMid: false, bbLower: false, rsi14: false, macd: false, atr14: false, stoch14: false,
      supertrend: false, avgprice: false, alma: false, adx: false, aroonUp: false, accdist: false, hi52: false, lo52: false,
    }
    if (preset === 'scalp') (['ema9', 'ema21', 'vwap', 'rsi14'] as StudyKey[]).forEach((k) => { next[k] = true })
    if (preset === 'swing') (['ema50', 'ema200', 'bbUpper', 'bbMid', 'bbLower', 'macd'] as StudyKey[]).forEach((k) => { next[k] = true })
    if (preset === 'trend') (['ema21', 'ema50', 'sma50', 'atr14', 'supertrend', 'adx'] as StudyKey[]).forEach((k) => { next[k] = true })
    setStudies(next)
    if (next.rsi14) setLowerPane('rsi')
    else if (next.macd) setLowerPane('macd')
    else if (next.atr14) setLowerPane('atr')
    else if (next.adx) setLowerPane('adx')
    else setLowerPane('none')
  }

  // Derived plot dimensions (used in render)

  return (
    <section ref={sectionRef} className="flex h-full min-h-0 min-w-0 flex-col rounded border border-[#2B2F36] bg-[#10141A]">
      <header className="flex shrink-0 items-center justify-between border-b border-[#2B2F36] px-1.5 py-[3px]">
        <div className="flex min-w-0 items-center gap-1">
          <div className="relative">
            <button className="inline-flex h-5 items-center gap-0.5 rounded border border-[#2B2F36] bg-[#0B0E11] px-1.5 text-[10px] text-[#D9DEE3]" onClick={() => setAssetMenuOpen((v) => !v)}>
              <Search size={10} />{symbol}<Plus size={8} />
            </button>
            {assetMenuOpen && (
              <div className="absolute left-0 top-6 z-40 w-44 rounded border border-[#2B2F36] bg-[#0B0E11] p-1 shadow-xl">
                <input value={assetSearch} onChange={(e) => setAssetSearch(e.target.value)} placeholder="Search symbol"
                  className={`mb-1 w-full rounded border border-[#2B2F36] bg-[#10141A] px-1.5 py-1 text-[10px] text-[#D9DEE3] outline-none ${monoClass}`} />
                <div className={`max-h-40 overflow-y-auto ${scrollClass}`}>
                  {filteredAssets.map((asset) => (
                    <button key={asset} className="block w-full rounded px-1.5 py-1 text-left text-[10px] text-[#D9DEE3] hover:bg-[#1C2128]"
                      onClick={() => { onSelectAsset?.(asset); setAssetMenuOpen(false); setAssetSearch('') }}>{asset}</button>
                  ))}
                </div>
              </div>
            )}
          </div>
          {timeframeOptions.slice(0, 4).map((tf) => (
            <button key={tf} className={`h-5 rounded px-1.5 text-[10px] ${interval === tf ? 'bg-[#1F2937] text-[#D9DEE3]' : 'text-[#AAB0B6]'}`} onClick={() => setInterval(tf)}>{tf}</button>
          ))}
          <button className="h-5 rounded px-1.5 text-[10px] text-[#AAB0B6]" onClick={openIndicatorModal}>Indicators</button>
          <button className="inline-flex h-5 w-5 items-center justify-center rounded text-[#AAB0B6] hover:text-[#D9DEE3]" onClick={handleUndo} title="Undo"><Undo2 size={10} /></button>
          <button className="inline-flex h-5 w-5 items-center justify-center rounded text-[#AAB0B6] hover:text-[#D9DEE3]" onClick={handleRedo} title="Redo"><Redo2 size={10} /></button>
          <span className="ml-0.5 truncate text-[10px] text-[#AAB0B6]">{symbol} · NSE</span>
          {latestCandle && (
            <span className={`hidden text-[10px] md:inline ${monoClass} ${delta >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>
              O{latestCandle.open.toFixed(2)} H{latestCandle.high.toFixed(2)} L{latestCandle.low.toFixed(2)} C{latestCandle.close.toFixed(2)} {delta >= 0 ? '+' : ''}{delta.toFixed(2)} ({delta >= 0 ? '+' : ''}{deltaPct.toFixed(2)}%)
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          <span className={`text-[12px] ${monoClass} ${tickDirection >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{lastPrice.toFixed(2)}</span>
          {isFullscreen && (
            <>
              <button className="h-5 rounded bg-[#00C076] px-2 text-[10px] font-semibold text-black hover:brightness-110 disabled:opacity-40" onClick={() => openQuickTrade('buy')} disabled={!onQuickTrade}>B</button>
              <button className="h-5 rounded bg-[#FF3B30] px-2 text-[10px] font-semibold text-white hover:brightness-110 disabled:opacity-40" onClick={() => openQuickTrade('sell')} disabled={!onQuickTrade}>S</button>
            </>
          )}
          <button className={`inline-flex h-5 w-5 items-center justify-center rounded ${showOverlays ? 'text-[#AAB0B6]' : 'text-[#FF3B30]'} hover:text-[#D9DEE3]`} onClick={() => setShowOverlays((v) => !v)} title="Toggle overlays"><Eye size={10} /></button>
          <button className="inline-flex h-5 w-5 items-center justify-center rounded text-[#AAB0B6] hover:text-[#D9DEE3]" onClick={handleScreenshot} title="Screenshot"><Camera size={10} /></button>
          <button className={`inline-flex h-5 w-5 items-center justify-center rounded ${isFullscreen ? 'text-[#00C076]' : 'text-[#AAB0B6]'} hover:text-[#D9DEE3]`} onClick={toggleFullscreen} title="Fullscreen"><Expand size={10} /></button>
        </div>
      </header>

      <div className="flex min-h-0 flex-1 gap-0.5 p-0.5">
        <aside className={`flex w-8 min-h-0 shrink-0 flex-col items-center gap-0.5 overflow-y-auto rounded border border-[#2B2F36] bg-[#0B0E11] py-1 ${scrollClass}`}>
          {toolGroups.map((group, gi) => (
            <div key={gi} className="flex flex-col items-center gap-0.5">
              {gi > 0 && <div className="my-0.5 h-px w-4 bg-[#2B2F36]" />}
              {group.map((tool) => (
                <button key={tool.key} title={tool.tip}
                  className={`inline-flex h-6 w-6 items-center justify-center rounded ${activeTool === tool.key ? 'bg-[#00C076] text-black' : 'text-[#AAB0B6] hover:bg-[#1C2128]'}`}
                  onClick={() => selectTool(tool.key)}>{tool.icon}</button>
              ))}
            </div>
          ))}
          <div className="my-0.5 h-px w-4 bg-[#2B2F36]" />
          <button title="Magnet" className={`inline-flex h-6 w-6 items-center justify-center rounded ${crosshairOn ? 'bg-[#00C076]/20 text-[#00C076]' : 'text-[#AAB0B6] hover:bg-[#1C2128]'}`} onClick={() => setCrosshairOn((v) => !v)}><Magnet size={13} /></button>
          <button title="Split View" className={`inline-flex h-6 w-6 items-center justify-center rounded ${splitView ? 'bg-[#00C076]/20 text-[#00C076]' : 'text-[#AAB0B6] hover:bg-[#1C2128]'}`} onClick={() => setSplitView((v) => !v)}><SplitSquareVertical size={13} /></button>
          <button title="Reset View" className="inline-flex h-6 w-6 items-center justify-center rounded text-[#AAB0B6] hover:bg-[#1C2128]" onClick={resetView}><RotateCcw size={13} /></button>
        </aside>

        <div className={`grid min-h-0 flex-1 ${lowerPane === 'none' ? 'grid-rows-[1fr_auto_auto]' : 'grid-rows-[1fr_auto_80px_auto]'} gap-0.5`}>
          <div className="flex flex-col min-h-0 relative gap-0.5">
            <div className="relative overflow-hidden rounded border border-[#2B2F36] min-h-0" style={{ flex: splitView ? `0 0 calc(${topHeightPct}% - 2.5px)` : '1 1 0%' }}>
              <div ref={primaryChartRef} className="h-full w-full" />
              {activeStudyChips.length > 0 && (
                <div className="pointer-events-none absolute left-1 top-1 z-30 flex flex-col">
                  {!overlayCollapsed && activeStudyChips.map((chip) => {
                    const study = overlayStudies.find((s) => s.key === chip.key)
                    const color = study?.color ?? '#AAB0B6'
                    const hidden = hiddenStudies.has(chip.key)
                    return (
                      <div key={chip.key} className="pointer-events-auto group flex items-center gap-1 rounded px-1 py-px hover:bg-[#1C2128]/60">
                        <Circle size={6} fill={color} stroke="none" className="shrink-0" />
                        <span className="text-[10px] text-[#AAB0B6]">{chip.label}</span>
                        <span className={`text-[10px] ${monoClass}`} style={{ color }}>{chip.value}</span>
                        <span className="ml-auto flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                          <button className="inline-flex h-4 w-4 items-center justify-center rounded text-[#AAB0B6] hover:text-[#D9DEE3]"
                            onClick={() => setHiddenStudies((prev) => { const next = new Set(prev); next.has(chip.key) ? next.delete(chip.key) : next.add(chip.key); return next })}>
                            {hidden ? <EyeOff size={10} /> : <Eye size={10} />}
                          </button>
                          <button className="inline-flex h-4 w-4 items-center justify-center rounded text-[#AAB0B6] hover:text-[#FF3B30]"
                            onClick={() => setStudies((prev) => ({ ...prev, [chip.key]: false }))}><Trash2 size={10} /></button>
                        </span>
                      </div>
                    )
                  })}
                  <button className="pointer-events-auto mt-0.5 flex items-center gap-0.5 rounded px-1 py-px text-[9px] text-[#6B7280] hover:text-[#AAB0B6]"
                    onClick={() => setOverlayCollapsed((v) => !v)}>
                    {overlayCollapsed ? <ChevronDown size={10} /> : <ChevronUp size={10} />}
                    {overlayCollapsed ? `${activeStudyChips.length} indicators` : 'collapse'}
                  </button>
                </div>
              )}

              {/* ─── SVG overlay ──────────────────────────────────────────────
                  FIX-2: The clipPath rect uses the live plotW/plotH values
                  (SVG total size minus the price-scale and time-scale gutters)
                  so drawings are strictly constrained to the candle plot area.
                  pointer-events is set to 'none' at the SVG level and only
                  the interactive <g> inside gets pointer-events:all — this
                  prevents the overlay from blocking lightweight-charts events
                  when no drawing tool is active.
              ──────────────────────────────────────────────────────────────── */}
              <svg
                ref={overlayRef}
                className="absolute inset-0 h-full w-full"
                style={{ pointerEvents: overlayInteractive ? 'all' : 'none', zIndex: overlayInteractive ? 20 : 10, cursor: overlayInteractive ? 'crosshair' : 'default' }}
                onClick={handleOverlayClick}
                onMouseMove={handleOverlayMove}
                onMouseDown={handleOverlayMouseDown}
                onMouseUp={handleOverlayMouseUp}
                onContextMenu={(e) => { e.preventDefault(); if (overlayInteractive) clearToolDrafts() }}
              >
                {showOverlays && overlayLines.map((ln) => {
                  const svgW = overlayRef.current?.clientWidth ?? 800
                  const svgH = overlayRef.current?.clientHeight ?? 500
                  const x1 = ln.logical1 !== undefined ? (apiRef.current?.timeScale().logicalToCoordinate(ln.logical1 as any) ?? ln.x1) : ln.x1
                  const y1 = ln.price1 !== undefined ? (seriesRef.current?.priceToCoordinate(ln.price1) ?? ln.y1) : ln.y1
                  const x2 = ln.logical2 !== undefined ? (apiRef.current?.timeScale().logicalToCoordinate(ln.logical2 as any) ?? ln.x2) : ln.x2
                  const y2 = ln.price2 !== undefined ? (seriesRef.current?.priceToCoordinate(ln.price2) ?? ln.y2) : ln.y2
                  const onLineClick = (e: ReactMouseEvent) => {
                    if (activeTool === 'eraser') {
                      e.stopPropagation(); pushUndoSnapshot(); setOverlayLines(prev => prev.filter(p => p.id !== ln.id)); return
                    }
                  }
                  if (ln.kind === 'arrow') {
                    const id = ln.id
                    const color = y2 < y1 ? '#00C076' : '#FF3B30'
                    const angle = Math.atan2(y2 - y1, x2 - x1)
                    const arrowSize = 10

                    // Offset endpoint slightly so arrowhead does not overlap line end
                    const offset = 2
                    const endX = x2 - Math.cos(angle) * offset
                    const endY = y2 - Math.sin(angle) * offset

                    const pt1X = endX - arrowSize * Math.cos(angle - Math.PI / 6)
                    const pt1Y = endY - arrowSize * Math.sin(angle - Math.PI / 6)
                    const pt2X = endX - arrowSize * Math.cos(angle + Math.PI / 6)
                    const pt2Y = endY - arrowSize * Math.sin(angle + Math.PI / 6)

                    return (
                      <g onClick={onLineClick} key={id} className="group cursor-pointer">
                        <line x1={x1} y1={y1} x2={Math.abs(endX - x1) < 1 ? x1 : endX} y2={Math.abs(endY - y1) < 1 ? y1 : endY} stroke="transparent" strokeWidth="20" strokeLinecap="round" />
                        <line
                          x1={x1} y1={y1} x2={Math.abs(endX - x1) < 1 ? x1 : endX} y2={Math.abs(endY - y1) < 1 ? y1 : endY}
                          stroke={color} strokeWidth="2" strokeLinecap="round" pointerEvents="none"
                        />
                        <polygon
                          points={`${endX},${endY} ${pt1X},${pt1Y} ${pt2X},${pt2Y}`}
                          fill={color}
                          className="transition-transform duration-200"
                          style={{ transformOrigin: `${endX}px ${endY}px` }}
                          pointerEvents="none"
                        />
                      </g>
                    )
                  }
                  if (ln.kind === 'ray') {
                    const [, , ex, ey] = extendLine(x1, y1, x2, y2, svgW, svgH, false)
                    return (
                      <g key={ln.id} onClick={onLineClick} className="cursor-pointer">
                        <line x1={x1} y1={y1} x2={ex} y2={ey} stroke="transparent" strokeWidth="20" />
                        <line x1={x1} y1={y1} x2={ex} y2={ey} stroke="#60A5FA" strokeWidth="1.5" strokeDasharray="6 3" pointerEvents="none" />
                      </g>
                    )
                  }
                  if (ln.kind === 'xline') {
                    const [sx, sy, ex, ey] = extendLine(x1, y1, x2, y2, svgW, svgH, true)
                    return (
                      <g key={ln.id} onClick={onLineClick} className="cursor-pointer">
                        <line x1={sx} y1={sy} x2={ex} y2={ey} stroke="transparent" strokeWidth="20" />
                        <line x1={sx} y1={sy} x2={ex} y2={ey} stroke="#60A5FA" strokeWidth="1.5" strokeDasharray="8 4" pointerEvents="none" />
                      </g>
                    )
                  }
                  const dash = ln.kind === 'hline' || ln.kind === 'vline' ? '4 2' : undefined
                  return (
                    <g key={ln.id} onClick={onLineClick} className="cursor-pointer">
                      <line x1={x1} y1={y1} x2={x2} y2={y2} stroke="transparent" strokeWidth="20" />
                      <line x1={x1} y1={y1} x2={x2} y2={y2} stroke="#60A5FA" strokeWidth="1.5" strokeDasharray={dash} pointerEvents="none" />
                    </g>
                  )
                })}
                {showOverlays && overlayRects.map((r) => {
                  const x1 = r.logical1 !== undefined ? (apiRef.current?.timeScale().logicalToCoordinate(r.logical1 as any) ?? r.x1) : r.x1
                  const y1 = r.price1 !== undefined ? (seriesRef.current?.priceToCoordinate(r.price1) ?? r.y1) : r.y1
                  const x2 = r.logical2 !== undefined ? (apiRef.current?.timeScale().logicalToCoordinate(r.logical2 as any) ?? r.x2) : r.x2
                  const y2 = r.price2 !== undefined ? (seriesRef.current?.priceToCoordinate(r.price2) ?? r.y2) : r.y2
                  return (
                    <rect onClick={(e) => { if (activeTool === 'eraser') { e.stopPropagation(); pushUndoSnapshot(); setOverlayRects(prev => prev.filter(p => p.id !== r.id)) } }} key={r.id} x={Math.min(x1, x2)} y={Math.min(y1, y2)} width={Math.abs(x2 - x1)} height={Math.abs(y2 - y1)} fill="rgba(96,165,250,0.08)" stroke="#60A5FA" strokeWidth="1" />
                  )
                })}
                {showOverlays && overlayFibs.map((f) => {
                  const x1 = f.logical1 !== undefined ? (apiRef.current?.timeScale().logicalToCoordinate(f.logical1 as any) ?? f.x1) : f.x1
                  const y1 = f.price1 !== undefined ? (seriesRef.current?.priceToCoordinate(f.price1) ?? f.y1) : f.y1
                  const x2 = f.logical2 !== undefined ? (apiRef.current?.timeScale().logicalToCoordinate(f.logical2 as any) ?? f.x2) : f.x2
                  const y2 = f.price2 !== undefined ? (seriesRef.current?.priceToCoordinate(f.price2) ?? f.y2) : f.y2
                  const top = Math.min(y1, y2), h = Math.abs(y2 - y1), left = Math.min(x1, x2), w = Math.abs(x2 - x1)
                  const levels = [0, 0.236, 0.382, 0.5, 0.618, 0.786, 1]
                  const colors = ['#EF4444', '#F59E0B', '#22C55E', '#3B82F6', '#22C55E', '#F59E0B', '#EF4444']
                  return (
                    <g key={f.id} onClick={(e) => { if (activeTool === 'eraser') { e.stopPropagation(); pushUndoSnapshot(); setOverlayFibs(prev => prev.filter(p => p.id !== f.id)) } }}>
                      {levels.map((lvl, idx) => {
                        const y = top + h * lvl
                        const nextY = idx < levels.length - 1 ? top + h * levels[idx + 1] : y
                        return (
                          <g key={lvl}>
                            {idx < levels.length - 1 && <rect x={left} y={y} width={w} height={nextY - y} fill={colors[idx]} fillOpacity="0.05" />}
                            <line x1={left} y1={y} x2={left + w} y2={y} stroke={colors[idx]} strokeWidth="1" />
                            <rect x={left + w - 48} y={y - 7} width="46" height="13" rx="2" fill="#0B0E11" fillOpacity="0.85" />
                            <text x={left + w - 44} y={y + 3} fill={colors[idx]} fontSize="9" fontFamily="monospace">{(lvl * 100).toFixed(1)}%</text>
                          </g>
                        )
                      })}
                    </g>
                  )
                })}
                {showOverlays && draftStart && hoverPoint && (activeTool === 'line' || activeTool === 'brush' || activeTool === 'ray' || activeTool === 'xline' || activeTool === 'arrow') && (
                  <line x1={draftStart.x} y1={draftStart.y} x2={hoverPoint.x} y2={hoverPoint.y} stroke="#93C5FD" strokeWidth="1.5" strokeDasharray="4 3" />
                )}
                {showOverlays && draftStart && hoverPoint && activeTool === 'rect' && (
                  <rect x={Math.min(draftStart.x, hoverPoint.x)} y={Math.min(draftStart.y, hoverPoint.y)} width={Math.abs(hoverPoint.x - draftStart.x)} height={Math.abs(hoverPoint.y - draftStart.y)} fill="rgba(96,165,250,0.06)" stroke="#93C5FD" strokeWidth="1" strokeDasharray="4 3" />
                )}
                {showOverlays && draftStart && hoverPoint && activeTool === 'fib' && (() => {
                  const ft = Math.min(draftStart.y, hoverPoint.y), fh = Math.abs(hoverPoint.y - draftStart.y), fl = Math.min(draftStart.x, hoverPoint.x), fw = Math.abs(hoverPoint.x - draftStart.x)
                  return (
                    <g>
                      {[0, 0.236, 0.382, 0.5, 0.618, 0.786, 1].map((lvl) => (
                        <line key={lvl} x1={fl} y1={ft + fh * lvl} x2={fl + fw} y2={ft + fh * lvl} stroke="#A78BFA" strokeWidth="0.6" strokeDasharray="4 3" opacity="0.6" />
                      ))}
                    </g>
                  )
                })()}
                {showOverlays && overlayLabels.map((label) => {
                  const x = label.logical !== undefined ? (apiRef.current?.timeScale().logicalToCoordinate(label.logical as any) ?? label.x) : label.x
                  const y = label.price !== undefined ? (seriesRef.current?.priceToCoordinate(label.price) ?? label.y) : label.y
                  const onClick = (e: ReactMouseEvent) => { if (activeTool === 'eraser') { e.stopPropagation(); pushUndoSnapshot(); setOverlayLabels((prev: OverlayLabel[]) => prev.filter(p => p.id !== label.id)) } }
                  const textMetrics = typeof window !== 'undefined' ? 50 : 50 // Rough wide hit area for text
                  return (
                    <g key={label.id} onClick={onClick} className="cursor-pointer">
                      <rect x={x - textMetrics / 2} y={y - 12} width={textMetrics} height={24} fill="transparent" />
                      <text x={x} y={y} fill="#D9DEE3" fontSize="11" style={{ userSelect: 'none', pointerEvents: 'none' }} textAnchor="middle" alignmentBaseline="middle">
                        {label.text}
                      </text>
                    </g>
                  )
                })}
                {showOverlays && measureStart && (measureEnd || hoverPoint) && (() => {
                  const end = measureEnd ?? hoverPoint!
                  const minX = Math.min(measureStart.x, end.x)
                  const maxX = Math.max(measureStart.x, end.x)
                  const minY = Math.min(measureStart.y, end.y)
                  const maxY = Math.max(measureStart.y, end.y)
                  const w = maxX - minX
                  const h = maxY - minY
                  
                  const svgW = overlayRef.current?.clientWidth ?? 800
                  const svgH = overlayRef.current?.clientHeight ?? 500
                  const rightScaleW = 56
                  
                  let tooltipLines = [`dx: ${w.toFixed(0)}px, dy: ${h.toFixed(0)}px`]
                  let isPositive = true
                  let topPriceStr = ''
                  let bottomPriceStr = ''
                  
                  try {
                    const price1 = seriesRef.current?.coordinateToPrice(measureStart.y)
                    const price2 = seriesRef.current?.coordinateToPrice(end.y)
                    const logical1 = apiRef.current?.timeScale().coordinateToLogical(measureStart.x)
                    const logical2 = apiRef.current?.timeScale().coordinateToLogical(end.x)
                    
                    if (price1 !== undefined && price1 !== null && price2 !== undefined && price2 !== null) {
                      const diff = price2 - price1
                      isPositive = diff >= 0
                      const pct = (diff / price1) * 100
                      const sign = diff >= 0 ? '+' : ''
                      const priceLine = `${sign}${diff.toFixed(2)} (${sign}${pct.toFixed(2)}%)`
                      
                      const pTop = measureStart.y < end.y ? price1 : price2
                      const pBottom = measureStart.y < end.y ? price2 : price1
                      topPriceStr = pTop.toFixed(2)
                      bottomPriceStr = pBottom.toFixed(2)
                      
                      let barsLine = ''
                      if (logical1 !== undefined && logical1 !== null && logical2 !== undefined && logical2 !== null) {
                        const bars = Math.abs(Math.round(logical2 - logical1))
                        const seconds = bars * timeframeToSeconds[interval]
                        let timeStr = ''
                        if (seconds < 60) timeStr = `${seconds}s`
                        else if (seconds < 3600) timeStr = `${Math.floor(seconds / 60)}m`
                        else if (seconds < 86400) timeStr = `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
                        else timeStr = `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`
                        
                        barsLine = `${bars} bars, ${timeStr}`
                      }
                      
                      tooltipLines = barsLine ? [priceLine, barsLine] : [priceLine]
                    }
                  } catch (e) {
                    // fallback to pixel dx/dy
                  }
                  
                  // exact TradingView colors
                  const baseColor = isPositive ? '#089981' : '#F23645'
                  const fillColor = isPositive ? 'rgba(8, 153, 129, 0.2)' : 'rgba(242, 54, 69, 0.2)'
                  const bgW = Math.max(...tooltipLines.map(t => t.length)) * 6.5 + 24
                  const bgH = tooltipLines.length * 16 + 12
                  
                  // Position tooltip below the box or above if no space
                  const centerX = minX + w / 2
                  const tipY = maxY + bgH + 10 < svgH ? maxY + 10 : minY - bgH - 10
                  
                  // Custom trig-based arrows
                  const renderArrow = (fromX: number, fromY: number, toX: number, toY: number) => {
                    if (Math.abs(fromX - toX) < 1 && Math.abs(fromY - toY) < 1) return null
                    const angle = Math.atan2(toY - fromY, toX - fromX)
                    const offset = 1
                    const ex = toX - Math.cos(angle) * offset
                    const ey = toY - Math.sin(angle) * offset
                    const arrSize = 8
                    const pt1X = ex - arrSize * Math.cos(angle - Math.PI / 6)
                    const pt1Y = ey - arrSize * Math.sin(angle - Math.PI / 6)
                    const pt2X = ex - arrSize * Math.cos(angle + Math.PI / 6)
                    const pt2Y = ey - arrSize * Math.sin(angle + Math.PI / 6)
                    return (
                      <g>
                        <line x1={fromX} y1={fromY} x2={ex} y2={ey} stroke={baseColor} strokeWidth="1.5" strokeLinecap="round" />
                        <polygon points={`${ex},${ey} ${pt1X},${pt1Y} ${pt2X},${pt2Y}`} fill={baseColor} />
                      </g>
                    )
                  }
                  
                  return (
                    <g>
                      {/* Box */}
                      <rect x={minX} y={minY} width={w} height={h} fill={fillColor} />
                      
                      {/* TV Arrows from start point to bounds */}
                      {renderArrow(measureStart.x, measureStart.y, end.x, measureStart.y)}
                      {renderArrow(measureStart.x, measureStart.y, measureStart.x, end.y)}

                      {/* Price Axis Highlight */}
                      {topPriceStr && bottomPriceStr && (
                        <>
                          <rect x={svgW - rightScaleW} y={minY} width={rightScaleW} height={maxY - minY} fill={fillColor} />
                          <line x1={maxX} y1={minY} x2={svgW} y2={minY} stroke={baseColor} strokeDasharray="2 4" strokeWidth="0.8" opacity="0.6" />
                          <line x1={maxX} y1={maxY} x2={svgW} y2={maxY} stroke={baseColor} strokeDasharray="2 4" strokeWidth="0.8" opacity="0.6" />
                          <rect x={svgW - rightScaleW} y={minY - 10} width={rightScaleW} height={20} fill={baseColor} />
                          <text x={svgW - rightScaleW / 2} y={minY + 4} fill="#FFF" fontSize="11" textAnchor="middle" fontFamily="system-ui, -apple-system, sans-serif">{topPriceStr}</text>
                          <rect x={svgW - rightScaleW} y={maxY - 10} width={rightScaleW} height={20} fill={baseColor} />
                          <text x={svgW - rightScaleW / 2} y={maxY + 4} fill="#FFF" fontSize="11" textAnchor="middle" fontFamily="system-ui, -apple-system, sans-serif">{bottomPriceStr}</text>
                        </>
                      )}
                      
                      {/* Tooltip Box */}
                      <rect x={centerX - bgW / 2} y={tipY} width={bgW} height={bgH} rx="4" fill={baseColor} opacity="0.9" />
                      {tooltipLines.map((line, i) => (
                        <text key={i} x={centerX} y={tipY + 16 + i * 16} fill="#FFFFFF" fontSize="12" textAnchor="middle" fontFamily="system-ui, -apple-system, sans-serif">{line}</text>
                      ))}
                    </g>
                  )
                })()}
              </svg>
            </div>
            {splitView && (
              <>
                <div 
                  className="h-[5px] shrink-0 cursor-row-resize bg-[#2B2F36] hover:bg-[#00C076]/80 transition-colors z-10"
                  onPointerDown={onSplitDragStart}
                />
                <div className="relative overflow-hidden rounded border border-[#2B2F36] min-h-0" style={{ flex: '1 1 0%' }}>
                  <div className="absolute top-1 left-1 z-30 flex items-center gap-1">
                    <div className="relative">
                      <button className="inline-flex h-5 items-center gap-0.5 rounded border border-[#2B2F36] bg-[#0B0E11]/80 px-1.5 text-[10px] text-[#D9DEE3]" onClick={() => setSecAssetMenuOpen((v) => !v)}>
                        <Search size={10} />{secondarySymbol}<Plus size={8} />
                      </button>
                      {secAssetMenuOpen && (
                        <div className="absolute left-0 top-6 z-40 w-44 rounded border border-[#2B2F36] bg-[#0B0E11] p-1 shadow-xl">
                          <input value={secAssetSearch} onChange={(e) => setSecAssetSearch(e.target.value)} placeholder="Search symbol"
                            className={`mb-1 w-full rounded border border-[#2B2F36] bg-[#10141A] px-1.5 py-1 text-[10px] text-[#D9DEE3] outline-none ${monoClass}`} />
                          <div className={`max-h-40 overflow-y-auto ${scrollClass}`}>
                            {filteredSecAssets.map((asset) => (
                              <button key={asset} className="block w-full rounded px-1.5 py-1 text-left text-[10px] text-[#D9DEE3] hover:bg-[#1C2128]"
                                onClick={() => { onSelectSecondaryAsset?.(asset); setSecAssetMenuOpen(false); setSecAssetSearch('') }}>{asset}</button>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                    <span className="mx-1 h-3 w-px bg-[#2B2F36]" />
                    {timeframeOptions.map((tf) => (
                      <button
                        key={`secondary-${tf}`}
                        className={`rounded px-1.5 py-[2px] text-[10px] shadow-sm transition-colors ${secondaryInterval === tf ? 'bg-[#00C076] text-black font-semibold' : 'bg-[#0B0E11]/80 text-[#AAB0B6] border border-[#2B2F36] hover:bg-[#2B2F36] hover:text-[#D9DEE3]'}`}
                        onClick={() => setSecondaryInterval(tf)}
                      >
                        {tf}
                      </button>
                    ))}
                  </div>
                  {secondaryActiveStudyChips.length > 0 && (
                    <div className="pointer-events-none absolute left-1 top-8 z-30 flex flex-col">
                      {!overlayCollapsed && secondaryActiveStudyChips.map((chip) => {
                        const study = overlayStudies.find((s) => s.key === chip.key)
                        const color = study?.color ?? '#AAB0B6'
                        const hidden = hiddenStudies.has(chip.key)
                        return (
                          <div key={chip.key} className="pointer-events-auto group flex items-center gap-1 rounded px-1 py-px hover:bg-[#1C2128]/60">
                            <Circle size={6} fill={color} stroke="none" className="shrink-0" />
                            <span className="text-[10px] text-[#AAB0B6]">{chip.label}</span>
                            <span className={`text-[10px] ${monoClass}`} style={{ color }}>{chip.value}</span>
                            <span className="ml-auto flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                              <button className="inline-flex h-4 w-4 items-center justify-center rounded text-[#AAB0B6] hover:text-[#D9DEE3]"
                                onClick={() => setHiddenStudies((prev) => { const next = new Set(prev); next.has(chip.key) ? next.delete(chip.key) : next.add(chip.key); return next })}>
                                {hidden ? <EyeOff size={10} /> : <Eye size={10} />}
                              </button>
                              <button className="inline-flex h-4 w-4 items-center justify-center rounded text-[#AAB0B6] hover:text-[#FF3B30]"
                                onClick={() => setStudies((prev) => ({ ...prev, [chip.key]: false }))}><Trash2 size={10} /></button>
                            </span>
                          </div>
                        )
                      })}
                    </div>
                  )}
                  <div ref={secondaryChartRef} className="h-full w-full" />
                </div>
              </>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-1.5 rounded border border-[#2B2F36] bg-[#0B0E11] px-2 py-1 text-[10px] text-[#AAB0B6]">
            <span className="text-[#D9DEE3]">Values</span>
            {studies.rsi14 && <span className={monoClass}>RSI {metrics.rsi14.toFixed(2)}</span>}
            {studies.macd && <span className={monoClass}>MACD {metrics.macd.line.toFixed(2)}/{metrics.macd.signal.toFixed(2)}/{metrics.macd.hist.toFixed(2)}</span>}
            {studies.atr14 && <span className={monoClass}>ATR {metrics.atr14.toFixed(2)}</span>}
            {studies.stoch14 && <span className={monoClass}>STOCH {metrics.stoch14.k.toFixed(2)}/{metrics.stoch14.d.toFixed(2)}</span>}
            {studies.supertrend && <span className={`${monoClass} ${metrics.supertrend.direction === 'UP' ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>ST {metrics.supertrend.value.toFixed(2)} {metrics.supertrend.direction}</span>}
            {studies.adx && <span className={monoClass}>ADX {metrics.adx.toFixed(2)}</span>}
            <span className="mx-1 h-3 w-px bg-[#2B2F36]" />
            {(['rsi', 'macd', 'atr', 'stoch', 'adx', 'aroon', 'accdist'] as Exclude<LowerPaneKey, 'none'>[]).map((paneKey) => (
              <button key={paneKey} className={`rounded px-1.5 py-[2px] ${lowerPane === paneKey ? 'bg-[#00C076]/20 text-[#00C076]' : 'bg-[#1C2128] text-[#AAB0B6]'}`}
                onClick={() => setLowerPane((p) => p === paneKey ? 'none' : paneKey)}>{paneKey.toUpperCase()}</button>
            ))}
          </div>

          {lowerPane !== 'none' && (
            <div className="rounded border border-[#2B2F36] bg-[#0B0E11]">
              <div className="border-b border-[#2B2F36] px-2 py-[2px] text-[10px] text-[#AAB0B6]">{lowerPaneLabel[lowerPane]}</div>
              <div ref={lowerPaneChartRef} className="h-[68px] w-full" />
            </div>
          )}
        </div>
      </div>

      {quickTrade && (
        <div className="absolute z-40 w-72 rounded border border-[#2B2F36] bg-[#10141A] shadow-2xl" style={{ left: quickTradePos.x, top: quickTradePos.y }}>
          <div className="flex cursor-move items-center justify-between border-b border-[#2B2F36] px-2 py-1.5"
            onMouseDown={(e) => { quickDragRef.current.dragging = true; quickDragRef.current.dx = e.clientX - quickTradePos.x; quickDragRef.current.dy = e.clientY - quickTradePos.y }}>
            <div>
              <div className="text-[11px] font-semibold text-[#D9DEE3]">{quickTrade.action === 'buy' ? 'Quick Buy' : 'Quick Sell'} · {symbol}</div>
              <div className={`text-[11px] ${monoClass} ${tickDirection >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>NSE ₹{lastPrice.toFixed(2)}</div>
            </div>
            <button className="inline-flex h-5 w-5 items-center justify-center rounded text-[#AAB0B6] hover:bg-[#1C2128]" onClick={() => setQuickTrade(null)}><X size={11} /></button>
          </div>
          <div className="space-y-2 p-2">
            <div className="grid grid-cols-2 gap-1 text-[10px]">
              <button className={`rounded border px-2 py-1 ${quickTrade.action === 'buy' ? 'border-[#00C076] bg-[#00C076]/20 text-[#00C076]' : 'border-[#2B2F36] bg-[#0B0E11] text-[#AAB0B6]'}`}
                onClick={() => setQuickTrade((p) => p ? { ...p, action: 'buy' } : p)}>BUY</button>
              <button className={`rounded border px-2 py-1 ${quickTrade.action === 'sell' ? 'border-[#FF3B30] bg-[#FF3B30]/20 text-[#FF3B30]' : 'border-[#2B2F36] bg-[#0B0E11] text-[#AAB0B6]'}`}
                onClick={() => setQuickTrade((p) => p ? { ...p, action: 'sell' } : p)}>SELL</button>
            </div>
            <div className="grid grid-cols-[1fr_1.1fr] items-center gap-1 text-[10px]">
              <span className="text-[#AAB0B6]">Qty (NSE)</span>
              <input className={`h-8 rounded border border-[#2B2F36] bg-[#0B0E11] px-2 text-right text-[11px] text-[#D9DEE3] ${monoClass}`}
                value={quickTrade.quantity} onChange={(e) => setQuickTrade((p) => p ? { ...p, quantity: e.target.value } : p)} />
              <span className="text-[#AAB0B6]">Price (INR)</span>
              <input className={`h-8 rounded border border-[#2B2F36] bg-[#0B0E11] px-2 text-right text-[11px] text-[#D9DEE3] disabled:opacity-50 ${monoClass}`}
                value={quickTrade.limitPrice} disabled={quickTrade.orderType === 'market'} onChange={(e) => setQuickTrade((p) => p ? { ...p, limitPrice: e.target.value } : p)} />
            </div>
            <div className="grid grid-cols-2 gap-1 text-[10px]">
              <button className={`rounded border px-2 py-1 ${quickTrade.orderType === 'market' ? 'border-[#F59E0B] bg-[#F59E0B]/15 text-[#F59E0B]' : 'border-[#2B2F36] bg-[#0B0E11] text-[#AAB0B6]'}`}
                onClick={() => setQuickTrade((p) => p ? { ...p, orderType: 'market', limitPrice: lastPrice.toFixed(2) } : p)}>Market</button>
              <button className={`rounded border px-2 py-1 ${quickTrade.orderType === 'limit' ? 'border-[#60A5FA] bg-[#60A5FA]/15 text-[#60A5FA]' : 'border-[#2B2F36] bg-[#0B0E11] text-[#AAB0B6]'}`}
                onClick={() => setQuickTrade((p) => p ? { ...p, orderType: 'limit' } : p)}>Limit</button>
            </div>
            <button className={`w-full rounded border px-2 py-1 text-[10px] ${quickTrade.useRisk ? 'border-[#60A5FA] bg-[#60A5FA]/15 text-[#60A5FA]' : 'border-[#2B2F36] bg-[#0B0E11] text-[#AAB0B6]'}`}
              onClick={() => setQuickTrade((p) => p ? { ...p, useRisk: !p.useRisk } : p)}>
              {quickTrade.useRisk ? 'Hide Stoploss / Target' : 'Add Stoploss / Target'}
            </button>
            {quickTrade.useRisk && (
              <div className="grid grid-cols-[1fr_1.1fr] items-center gap-1 text-[10px]">
                <span className="text-[#AAB0B6]">Stoploss</span>
                <input className={`h-8 rounded border border-[#2B2F36] bg-[#0B0E11] px-2 text-right text-[11px] text-[#D9DEE3] ${monoClass}`}
                  value={quickTrade.stopLoss} placeholder="e.g. 210.50" onChange={(e) => setQuickTrade((p) => p ? { ...p, stopLoss: e.target.value } : p)} />
                <span className="text-[#AAB0B6]">Target</span>
                <input className={`h-8 rounded border border-[#2B2F36] bg-[#0B0E11] px-2 text-right text-[11px] text-[#D9DEE3] ${monoClass}`}
                  value={quickTrade.target} placeholder="e.g. 218.00" onChange={(e) => setQuickTrade((p) => p ? { ...p, target: e.target.value } : p)} />
              </div>
            )}
            <div className="rounded border border-[#2B2F36] bg-[#0B0E11] px-2 py-1 text-[10px] text-[#AAB0B6]">
              Est. Notional: <span className={`${monoClass} text-[#D9DEE3]`}>₹{(Number(quickTrade.quantity || 0) * (quickTrade.orderType === 'market' ? lastPrice : Number(quickTrade.limitPrice || 0))).toFixed(2)}</span>
            </div>
            {quickTradeError && <div className="rounded border border-[#FF3B30]/30 bg-[#FF3B30]/10 px-2 py-1 text-[10px] text-[#FF7C74]">{quickTradeError}</div>}
            <button className={`w-full rounded px-2 py-2 text-[11px] font-semibold disabled:cursor-not-allowed disabled:opacity-45 ${quickTrade.action === 'buy' ? 'bg-[#00C076] text-black' : 'bg-[#FF3B30] text-white'}`}
              onClick={submitQuickTrade} disabled={Boolean(quickTradeError)}>
              Confirm {quickTrade.action === 'buy' ? 'Buy' : 'Sell'}
            </button>
          </div>
        </div>
      )}

      {indicatorModalOpen && (
        <div className="absolute inset-0 z-30 bg-black/50 p-4" onMouseDown={() => setIndicatorModalOpen(false)}>
          <div className="absolute flex h-[72%] w-[430px] flex-col overflow-hidden rounded border border-[#2B2F36] bg-[#0F1218] shadow-2xl"
            style={{ left: modalPosition?.x ?? 24, top: modalPosition?.y ?? 24 }} onMouseDown={(e) => e.stopPropagation()}>
            <div className="flex cursor-move items-center justify-between border-b border-[#2B2F36] px-4 py-3" onMouseDown={startModalDrag}>
              <h3 className="text-[18px] font-semibold text-[#D9DEE3]">Indicators</h3>
              <button className="inline-flex h-8 w-8 items-center justify-center rounded bg-[#1C2128] text-[#AAB0B6]" onClick={() => setIndicatorModalOpen(false)}><X size={16} /></button>
            </div>
            <div className="border-b border-[#2B2F36] p-3">
              <label className="flex items-center gap-2 rounded border border-[#2B2F36] bg-[#0B0E11] px-3 py-2 text-[#AAB0B6]">
                <Search size={14} />
                <input value={indicatorSearch} onChange={(e) => setIndicatorSearch(e.target.value)}
                  className="w-full bg-transparent text-[13px] outline-none placeholder:text-[#657080]" placeholder="Search" />
              </label>
              <div className="mt-2 flex gap-1">
                {(['all', 'favorites', 'builtins'] as IndicatorTab[]).map((tab) => (
                  <button key={tab} className={`rounded px-2 py-[3px] text-[10px] ${indicatorTab === tab ? 'bg-[#1F2937] text-[#D9DEE3]' : 'bg-[#1C2128] text-[#AAB0B6]'}`}
                    onClick={() => setIndicatorTab(tab)}>{tab.charAt(0).toUpperCase() + tab.slice(1)}</button>
                ))}
              </div>
              <div className="mt-2 flex gap-1">
                {(['scalp', 'swing', 'trend'] as const).map((p) => (
                  <button key={p} className="rounded bg-[#1C2128] px-2 py-[3px] text-[10px] text-[#AAB0B6]" onClick={() => applyPreset(p)}>{p.charAt(0).toUpperCase() + p.slice(1)}</button>
                ))}
              </div>
            </div>
            <div className={`min-h-0 flex-1 overflow-y-auto p-2 ${scrollClass}`}>
              {filteredIndicators.length === 0 && (
                <div className="rounded border border-[#2B2F36] bg-[#0B0E11] px-3 py-2 text-[12px] text-[#6B7280]">No indicators found for this filter.</div>
              )}
              {filteredIndicators.map((item) => {
                const enabled = isStudyEnabled(item)
                const favorite = favorites.includes(item.id)
                return (
                  <div key={item.id} className="mb-1 flex items-center gap-2 rounded px-2 py-2 hover:bg-[#1C2128]">
                    <button className={`text-[12px] ${favorite ? 'text-[#FCD34D]' : 'text-[#6B7280]'}`} onClick={() => toggleFavorite(item.id)}>{favorite ? '*' : 'o'}</button>
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[13px] text-[#D9DEE3]">{item.label}</div>
                      <div className="text-[10px] text-[#6B7280]">{item.category}</div>
                    </div>
                    <button className={`rounded px-2 py-[2px] text-[10px] ${enabled ? 'bg-[#00C076]/20 text-[#00C076]' : 'bg-[#1C2128] text-[#AAB0B6]'}`}
                      onClick={() => toggleStudy(item)}>{enabled ? 'Added' : 'Add'}</button>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      )}
    </section>
  )
}