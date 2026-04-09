/* ── Alpha Bot – Types & Node Registry ──────────────────────── */

// ─── Node categories ───────────────────────────────────────
export type NodeCategory = 'data' | 'indicator' | 'condition' | 'action'

// ─── All supported node types ──────────────────────────────
export type NodeType =
  | 'priceFeed'
  | 'sma'
  | 'ema'
  | 'rsi'
  | 'macd'
  | 'bollingerBands'
  | 'crossover'
  | 'threshold'
  | 'and'
  | 'or'
  | 'marketBuy'
  | 'marketSell'
  | 'stopLoss'

// ─── Port definitions ──────────────────────────────────────
export type PortDef = {
  id: string
  label: string
  /** 'number' streams (price/indicator values) or 'boolean' (signal) */
  dataType: 'number' | 'boolean'
}

// ─── Param definitions (for auto-generated config forms) ───
export type ParamDef = {
  key: string
  label: string
  type: 'number' | 'select'
  default: number | string
  min?: number
  max?: number
  step?: number
  options?: { label: string; value: string }[]
}

// ─── Node registry entry (static metadata) ─────────────────
export type NodeRegistryEntry = {
  type: NodeType
  label: string
  category: NodeCategory
  color: string // Tailwind-friendly hex
  inputs: PortDef[]
  outputs: PortDef[]
  params: ParamDef[]
}

// ─── Runtime node instance ──────────────────────────────────
export type BotNode = {
  id: string
  type: NodeType
  x: number
  y: number
  params: Record<string, number | string>
  label: string
}

// ─── Edge between two node ports ────────────────────────────
export type BotEdge = {
  id: string
  fromNode: string
  fromPort: string
  toNode: string
  toPort: string
}

// ─── Full strategy graph ────────────────────────────────────
export type BotStrategy = {
  nodes: BotNode[]
  edges: BotEdge[]
}

// ─── Node registry ──────────────────────────────────────────
export const NODE_REGISTRY: NodeRegistryEntry[] = [
  // ── Data Sources ──
  {
    type: 'priceFeed',
    label: 'Price Feed',
    category: 'data',
    color: '#00C076',
    inputs: [],
    outputs: [{ id: 'price', label: 'Price', dataType: 'number' }],
    params: [],
  },

  // ── Indicators ──
  {
    type: 'sma',
    label: 'SMA',
    category: 'indicator',
    color: '#3B82F6',
    inputs: [{ id: 'value', label: 'Value', dataType: 'number' }],
    outputs: [{ id: 'result', label: 'SMA', dataType: 'number' }],
    params: [
      { key: 'period', label: 'Period', type: 'number', default: 20, min: 2, max: 200, step: 1 },
    ],
  },
  {
    type: 'ema',
    label: 'EMA',
    category: 'indicator',
    color: '#3B82F6',
    inputs: [{ id: 'value', label: 'Value', dataType: 'number' }],
    outputs: [{ id: 'result', label: 'EMA', dataType: 'number' }],
    params: [
      { key: 'period', label: 'Period', type: 'number', default: 12, min: 2, max: 200, step: 1 },
    ],
  },
  {
    type: 'rsi',
    label: 'RSI',
    category: 'indicator',
    color: '#3B82F6',
    inputs: [{ id: 'value', label: 'Value', dataType: 'number' }],
    outputs: [{ id: 'result', label: 'RSI', dataType: 'number' }],
    params: [
      { key: 'period', label: 'Period', type: 'number', default: 14, min: 2, max: 100, step: 1 },
    ],
  },
  {
    type: 'macd',
    label: 'MACD',
    category: 'indicator',
    color: '#3B82F6',
    inputs: [{ id: 'value', label: 'Value', dataType: 'number' }],
    outputs: [
      { id: 'macdLine', label: 'MACD', dataType: 'number' },
      { id: 'signalLine', label: 'Signal', dataType: 'number' },
    ],
    params: [
      { key: 'fastPeriod', label: 'Fast', type: 'number', default: 12, min: 2, max: 100, step: 1 },
      { key: 'slowPeriod', label: 'Slow', type: 'number', default: 26, min: 2, max: 200, step: 1 },
      { key: 'signalPeriod', label: 'Signal', type: 'number', default: 9, min: 2, max: 100, step: 1 },
    ],
  },
  {
    type: 'bollingerBands',
    label: 'Bollinger Bands',
    category: 'indicator',
    color: '#3B82F6',
    inputs: [{ id: 'value', label: 'Value', dataType: 'number' }],
    outputs: [
      { id: 'upper', label: 'Upper', dataType: 'number' },
      { id: 'mid', label: 'Mid', dataType: 'number' },
      { id: 'lower', label: 'Lower', dataType: 'number' },
    ],
    params: [
      { key: 'period', label: 'Period', type: 'number', default: 20, min: 2, max: 200, step: 1 },
      { key: 'stdDev', label: 'Std Dev', type: 'number', default: 2, min: 0.5, max: 5, step: 0.1 },
    ],
  },

  // ── Conditions ──
  {
    type: 'crossover',
    label: 'Crossover',
    category: 'condition',
    color: '#F0B90B',
    inputs: [
      { id: 'fast', label: 'Fast', dataType: 'number' },
      { id: 'slow', label: 'Slow', dataType: 'number' },
    ],
    outputs: [
      { id: 'crossUp', label: 'Cross Up', dataType: 'boolean' },
      { id: 'crossDown', label: 'Cross Down', dataType: 'boolean' },
    ],
    params: [],
  },
  {
    type: 'threshold',
    label: 'Threshold',
    category: 'condition',
    color: '#F0B90B',
    inputs: [{ id: 'value', label: 'Value', dataType: 'number' }],
    outputs: [{ id: 'signal', label: 'Signal', dataType: 'boolean' }],
    params: [
      {
        key: 'operator',
        label: 'Operator',
        type: 'select',
        default: '>',
        options: [
          { label: '>', value: '>' },
          { label: '<', value: '<' },
          { label: '>=', value: '>=' },
          { label: '<=', value: '<=' },
        ],
      },
      { key: 'value', label: 'Threshold', type: 'number', default: 50, min: -999999, max: 999999, step: 0.01 },
    ],
  },
  {
    type: 'and',
    label: 'AND',
    category: 'condition',
    color: '#F0B90B',
    inputs: [
      { id: 'a', label: 'A', dataType: 'boolean' },
      { id: 'b', label: 'B', dataType: 'boolean' },
    ],
    outputs: [{ id: 'result', label: 'Result', dataType: 'boolean' }],
    params: [],
  },
  {
    type: 'or',
    label: 'OR',
    category: 'condition',
    color: '#F0B90B',
    inputs: [
      { id: 'a', label: 'A', dataType: 'boolean' },
      { id: 'b', label: 'B', dataType: 'boolean' },
    ],
    outputs: [{ id: 'result', label: 'Result', dataType: 'boolean' }],
    params: [],
  },

  // ── Actions ──
  {
    type: 'marketBuy',
    label: 'Market Buy',
    category: 'action',
    color: '#00C076',
    inputs: [{ id: 'trigger', label: 'Trigger', dataType: 'boolean' }],
    outputs: [],
    params: [
      { key: 'quantity', label: 'Quantity', type: 'number', default: 0.01, min: 0.001, max: 100, step: 0.001 },
    ],
  },
  {
    type: 'marketSell',
    label: 'Market Sell',
    category: 'action',
    color: '#FF3B30',
    inputs: [{ id: 'trigger', label: 'Trigger', dataType: 'boolean' }],
    outputs: [],
    params: [
      { key: 'quantity', label: 'Quantity', type: 'number', default: 0.01, min: 0.001, max: 100, step: 0.001 },
    ],
  },
  {
    type: 'stopLoss',
    label: 'Stop Loss',
    category: 'action',
    color: '#FF3B30',
    inputs: [{ id: 'trigger', label: 'Trigger', dataType: 'boolean' }],
    outputs: [],
    params: [
      { key: 'threshold', label: '% Drop', type: 'number', default: 2, min: 0.1, max: 50, step: 0.1 },
      { key: 'quantity', label: 'Quantity', type: 'number', default: 0.01, min: 0.001, max: 100, step: 0.001 },
    ],
  },
]

export const getRegistryEntry = (type: NodeType): NodeRegistryEntry =>
  NODE_REGISTRY.find((e) => e.type === type)!
