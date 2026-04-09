/* ── Alpha Bot – Strategy Engine Hook ──────────────────────── */
import { useCallback, useEffect, useRef, useState } from 'react'
import type { BotNode, BotEdge, BotStrategy, NodeType } from '../types/alphaBot'
import { getRegistryEntry } from '../types/alphaBot'
import type { MarketFeed } from './useMockMarket'

// ─── Indicator math helpers ────────────────────────────────
const computeSMA = (buf: number[], period: number): number | null => {
  if (buf.length < period) return null
  let sum = 0
  for (let i = buf.length - period; i < buf.length; i++) sum += buf[i]
  return sum / period
}

const computeEMA = (buf: number[], period: number): number | null => {
  if (buf.length < period) return null
  const k = 2 / (period + 1)
  let ema = buf[buf.length - period]
  for (let i = buf.length - period + 1; i < buf.length; i++) {
    ema = buf[i] * k + ema * (1 - k)
  }
  return ema
}

const computeRSI = (buf: number[], period: number): number | null => {
  if (buf.length < period + 1) return null
  let gains = 0
  let losses = 0
  const start = buf.length - period - 1
  for (let i = start + 1; i < buf.length; i++) {
    const diff = buf[i] - buf[i - 1]
    if (diff > 0) gains += diff
    else losses -= diff
  }
  const avgGain = gains / period
  const avgLoss = losses / period
  if (avgLoss === 0) return 100
  const rs = avgGain / avgLoss
  return 100 - 100 / (1 + rs)
}

const computeMACD = (
  buf: number[],
  fast: number,
  slow: number,
  signal: number,
): { macdLine: number; signalLine: number } | null => {
  const emaFast = computeEMA(buf, fast)
  const emaSlow = computeEMA(buf, slow)
  if (emaFast === null || emaSlow === null) return null
  const macdLine = emaFast - emaSlow
  // Approximate signal as SMA of recent MACD values — simplified for real-time
  // In a full implementation you'd keep a rolling MACD buffer; here we just use the
  // EMA of the price difference as a reasonable approximation
  const signalBuf: number[] = []
  for (let i = Math.max(slow, buf.length - signal * 2); i < buf.length; i++) {
    const f = computeEMA(buf.slice(0, i + 1), fast)
    const s = computeEMA(buf.slice(0, i + 1), slow)
    if (f !== null && s !== null) signalBuf.push(f - s)
  }
  const signalLine = signalBuf.length >= signal ? computeSMA(signalBuf, signal) ?? macdLine : macdLine
  return { macdLine, signalLine }
}

const computeBollinger = (buf: number[], period: number, mult: number) => {
  const sma = computeSMA(buf, period)
  if (sma === null) return null
  let variance = 0
  for (let i = buf.length - period; i < buf.length; i++) {
    variance += (buf[i] - sma) ** 2
  }
  const std = Math.sqrt(variance / period)
  return { upper: sma + mult * std, mid: sma, lower: sma - mult * std }
}

// ─── Topological ordering ──────────────────────────────────
const topoSort = (nodes: BotNode[], edges: BotEdge[]): BotNode[] => {
  const inDeg = new Map<string, number>()
  const adj = new Map<string, string[]>()
  for (const n of nodes) {
    inDeg.set(n.id, 0)
    adj.set(n.id, [])
  }
  for (const e of edges) {
    adj.get(e.fromNode)?.push(e.toNode)
    inDeg.set(e.toNode, (inDeg.get(e.toNode) ?? 0) + 1)
  }
  const queue: string[] = []
  for (const [id, deg] of inDeg) if (deg === 0) queue.push(id)
  const sorted: string[] = []
  while (queue.length > 0) {
    const id = queue.shift()!
    sorted.push(id)
    for (const next of adj.get(id) ?? []) {
      const d = (inDeg.get(next) ?? 1) - 1
      inDeg.set(next, d)
      if (d === 0) queue.push(next)
    }
  }
  const nodeMap = new Map(nodes.map((n) => [n.id, n]))
  return sorted.map((id) => nodeMap.get(id)!).filter(Boolean)
}

// ─── Unique ID helper ──────────────────────────────────────
let _uid = 0
export const uid = () => `node_${++_uid}_${Date.now().toString(36)}`

// ─── Create a new node from a type ─────────────────────────
export const createNode = (type: NodeType, x: number, y: number): BotNode => {
  const entry = getRegistryEntry(type)
  const params: Record<string, number | string> = {}
  for (const p of entry.params) params[p.key] = p.default
  return { id: uid(), type, x, y, params, label: entry.label }
}

// ─── Bot status ────────────────────────────────────────────
export type BotStatus = 'idle' | 'running' | 'error'

export type BotLog = {
  id: number
  time: number
  message: string
  type: 'info' | 'trade' | 'error'
}

export type BotPreset = 'scalper' | 'meanReversion' | 'breakout'

const presetLabel = (preset: BotPreset) =>
  preset === 'scalper' ? 'Scalper' : preset === 'meanReversion' ? 'Mean Reversion' : 'Breakout'

const createPresetGraph = (preset: BotPreset): BotStrategy => {
  if (preset === 'scalper') {
    const price = createNode('priceFeed', 80, 120)
    const emaFast = createNode('ema', 320, 60)
    emaFast.params.period = 9
    const emaSlow = createNode('ema', 320, 200)
    emaSlow.params.period = 21
    const cross = createNode('crossover', 560, 130)
    const buy = createNode('marketBuy', 780, 70)
    buy.params.quantity = 1
    const sell = createNode('marketSell', 780, 190)
    sell.params.quantity = 1
    return {
      nodes: [price, emaFast, emaSlow, cross, buy, sell],
      edges: [
        { id: uid(), fromNode: emaFast.id, fromPort: 'result', toNode: cross.id, toPort: 'fast' },
        { id: uid(), fromNode: emaSlow.id, fromPort: 'result', toNode: cross.id, toPort: 'slow' },
        { id: uid(), fromNode: cross.id, fromPort: 'crossUp', toNode: buy.id, toPort: 'trigger' },
        { id: uid(), fromNode: cross.id, fromPort: 'crossDown', toNode: sell.id, toPort: 'trigger' },
      ],
    }
  }
  if (preset === 'meanReversion') {
    const price = createNode('priceFeed', 80, 120)
    const rsi = createNode('rsi', 300, 120)
    rsi.params.period = 14
    const low = createNode('threshold', 520, 70)
    low.params.operator = '<='
    low.params.value = 30
    const high = createNode('threshold', 520, 180)
    high.params.operator = '>='
    high.params.value = 70
    const buy = createNode('marketBuy', 760, 60)
    buy.params.quantity = 1
    const sell = createNode('marketSell', 760, 190)
    sell.params.quantity = 1
    return {
      nodes: [price, rsi, low, high, buy, sell],
      edges: [
        { id: uid(), fromNode: rsi.id, fromPort: 'result', toNode: low.id, toPort: 'value' },
        { id: uid(), fromNode: rsi.id, fromPort: 'result', toNode: high.id, toPort: 'value' },
        { id: uid(), fromNode: low.id, fromPort: 'signal', toNode: buy.id, toPort: 'trigger' },
        { id: uid(), fromNode: high.id, fromPort: 'signal', toNode: sell.id, toPort: 'trigger' },
      ],
    }
  }
  const price = createNode('priceFeed', 80, 120)
  const sma = createNode('sma', 300, 120)
  sma.params.period = 55
  const threshold = createNode('threshold', 520, 120)
  threshold.params.operator = '>'
  threshold.params.value = 0
  const buy = createNode('marketBuy', 740, 90)
  buy.params.quantity = 1
  const stop = createNode('stopLoss', 740, 180)
  stop.params.threshold = 1.8
  stop.params.quantity = 1
  return {
    nodes: [price, sma, threshold, buy, stop],
    edges: [
      { id: uid(), fromNode: sma.id, fromPort: 'result', toNode: threshold.id, toPort: 'value' },
      { id: uid(), fromNode: threshold.id, fromPort: 'signal', toNode: buy.id, toPort: 'trigger' },
      { id: uid(), fromNode: threshold.id, fromPort: 'signal', toNode: stop.id, toPort: 'trigger' },
    ],
  }
}

// ─── The hook ──────────────────────────────────────────────
export const useAlphaBot = (feed: MarketFeed, symbol: string) => {
  const [nodes, setNodes] = useState<BotNode[]>([])
  const [edges, setEdges] = useState<BotEdge[]>([])
  const [status, setStatus] = useState<BotStatus>('idle')
  const [logs, setLogs] = useState<BotLog[]>([])
  const [pnl, setPnl] = useState(0)
  const [selectedPreset, setSelectedPreset] = useState<BotPreset>('scalper')

  const logId = useRef(0)
  const priceBufferRef = useRef<number[]>([])
  const prevValuesRef = useRef<Map<string, number | boolean>>(new Map())
  const cooldownRef = useRef(0)
  const runningRef = useRef(false)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const entryPriceRef = useRef(0)
  const nodesRef = useRef(nodes)
  const edgesRef = useRef(edges)

  useEffect(() => { nodesRef.current = nodes }, [nodes])
  useEffect(() => { edgesRef.current = edges }, [edges])

  const addLog = useCallback((message: string, type: BotLog['type'] = 'info') => {
    logId.current++
    const entry: BotLog = { id: logId.current, time: Date.now(), message, type }
    setLogs((prev) => [entry, ...prev].slice(0, 50))
  }, [])

  // ── Node / Edge CRUD ──
  const addNode = useCallback((node: BotNode) => {
    setNodes((prev) => [...prev, node])
  }, [])

  const removeNode = useCallback((id: string) => {
    setNodes((prev) => prev.filter((n) => n.id !== id))
    setEdges((prev) => prev.filter((e) => e.fromNode !== id && e.toNode !== id))
  }, [])

  const updateNodeParams = useCallback((id: string, params: Record<string, number | string>) => {
    setNodes((prev) =>
      prev.map((n) => (n.id === id ? { ...n, params: { ...n.params, ...params } } : n)),
    )
  }, [])

  const moveNode = useCallback((id: string, x: number, y: number) => {
    setNodes((prev) => prev.map((n) => (n.id === id ? { ...n, x, y } : n)))
  }, [])

  const addEdge = useCallback((edge: BotEdge) => {
    setEdges((prev) => {
      // Prevent duplicate connections to same input port
      const filtered = prev.filter(
        (e) => !(e.toNode === edge.toNode && e.toPort === edge.toPort),
      )
      return [...filtered, edge]
    })
  }, [])

  const removeEdge = useCallback((id: string) => {
    setEdges((prev) => prev.filter((e) => e.id !== id))
  }, [])

  const clearAll = useCallback(() => {
    stopBot()
    setNodes([])
    setEdges([])
    setLogs([])
    setPnl(0)
    priceBufferRef.current = []
    prevValuesRef.current.clear()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // ── Evaluate one tick of the strategy graph ──
  const evaluateTick = useCallback(() => {
    const currentNodes = nodesRef.current
    const currentEdges = edgesRef.current
    if (currentNodes.length === 0) return

    const snapshot = feed.getSnapshot()
    const price = snapshot.lastPrice
    const buf = priceBufferRef.current
    buf.push(price)
    if (buf.length > 500) buf.shift()

    if (entryPriceRef.current === 0) entryPriceRef.current = price

    const sorted = topoSort(currentNodes, currentEdges)
    const portValues = new Map<string, number | boolean>()

    for (const node of sorted) {
      // Gather inputs
      const inputs = new Map<string, number | boolean>()
      for (const edge of currentEdges) {
        if (edge.toNode === node.id) {
          const val = portValues.get(`${edge.fromNode}.${edge.fromPort}`)
          if (val !== undefined) inputs.set(edge.toPort, val)
        }
      }

      const p = node.params
      switch (node.type) {
        case 'priceFeed':
          portValues.set(`${node.id}.price`, price)
          break

        case 'sma': {
          const v = computeSMA(buf, Number(p.period) || 20)
          if (v !== null) portValues.set(`${node.id}.result`, v)
          break
        }

        case 'ema': {
          const v = computeEMA(buf, Number(p.period) || 12)
          if (v !== null) portValues.set(`${node.id}.result`, v)
          break
        }

        case 'rsi': {
          const v = computeRSI(buf, Number(p.period) || 14)
          if (v !== null) portValues.set(`${node.id}.result`, v)
          break
        }

        case 'macd': {
          const v = computeMACD(
            buf,
            Number(p.fastPeriod) || 12,
            Number(p.slowPeriod) || 26,
            Number(p.signalPeriod) || 9,
          )
          if (v) {
            portValues.set(`${node.id}.macdLine`, v.macdLine)
            portValues.set(`${node.id}.signalLine`, v.signalLine)
          }
          break
        }

        case 'bollingerBands': {
          const v = computeBollinger(buf, Number(p.period) || 20, Number(p.stdDev) || 2)
          if (v) {
            portValues.set(`${node.id}.upper`, v.upper)
            portValues.set(`${node.id}.mid`, v.mid)
            portValues.set(`${node.id}.lower`, v.lower)
          }
          break
        }

        case 'crossover': {
          const fast = inputs.get('fast')
          const slow = inputs.get('slow')
          if (typeof fast === 'number' && typeof slow === 'number') {
            const prevFast = prevValuesRef.current.get(`${node.id}.fast`)
            const prevSlow = prevValuesRef.current.get(`${node.id}.slow`)
            const crossUp =
              typeof prevFast === 'number' &&
              typeof prevSlow === 'number' &&
              prevFast <= prevSlow &&
              fast > slow
            const crossDown =
              typeof prevFast === 'number' &&
              typeof prevSlow === 'number' &&
              prevFast >= prevSlow &&
              fast < slow
            portValues.set(`${node.id}.crossUp`, crossUp)
            portValues.set(`${node.id}.crossDown`, crossDown)
            prevValuesRef.current.set(`${node.id}.fast`, fast)
            prevValuesRef.current.set(`${node.id}.slow`, slow)
          }
          break
        }

        case 'threshold': {
          const val = inputs.get('value')
          if (typeof val === 'number') {
            const op = String(p.operator)
            const thresh = Number(p.value)
            let result = false
            if (op === '>') result = val > thresh
            else if (op === '<') result = val < thresh
            else if (op === '>=') result = val >= thresh
            else if (op === '<=') result = val <= thresh
            portValues.set(`${node.id}.signal`, result)
          }
          break
        }

        case 'and': {
          const a = inputs.get('a')
          const b = inputs.get('b')
          portValues.set(`${node.id}.result`, a === true && b === true)
          break
        }

        case 'or': {
          const a = inputs.get('a')
          const b = inputs.get('b')
          portValues.set(`${node.id}.result`, a === true || b === true)
          break
        }

        case 'marketBuy': {
          const trigger = inputs.get('trigger')
          if (trigger === true && Date.now() > cooldownRef.current) {
            const qty = Number(p.quantity) || 0.01
            feed.executeOrder({
              asset: symbol,
              action: 'buy',
              direction: 'long',
              quantity: qty,
              orderType: 'market',
              limitPrice: price,
              timestamp: Date.now(),
            })
            cooldownRef.current = Date.now() + 1000 // 1s cooldown
            addLog(`BUY ${qty} @ ${price.toFixed(2)}`, 'trade')
          }
          break
        }

        case 'marketSell': {
          const trigger = inputs.get('trigger')
          if (trigger === true && Date.now() > cooldownRef.current) {
            const qty = Number(p.quantity) || 0.01
            feed.executeOrder({
              asset: symbol,
              action: 'sell',
              direction: 'short',
              quantity: qty,
              orderType: 'market',
              limitPrice: price,
              timestamp: Date.now(),
            })
            cooldownRef.current = Date.now() + 1000
            addLog(`SELL ${qty} @ ${price.toFixed(2)}`, 'trade')
          }
          break
        }

        case 'stopLoss': {
          const trigger = inputs.get('trigger')
          if (trigger === true && entryPriceRef.current > 0) {
            const dropPct = ((entryPriceRef.current - price) / entryPriceRef.current) * 100
            const thresh = Number(p.threshold) || 2
            if (dropPct >= thresh && Date.now() > cooldownRef.current) {
              const qty = Number(p.quantity) || 0.01
              feed.executeOrder({
                asset: symbol,
                action: 'sell',
                direction: 'short',
                quantity: qty,
                orderType: 'market',
                limitPrice: price,
                timestamp: Date.now(),
              })
              cooldownRef.current = Date.now() + 3000
              addLog(`STOP LOSS triggered (−${dropPct.toFixed(1)}%) SELL ${qty} @ ${price.toFixed(2)}`, 'trade')
            }
          }
          break
        }
      }
    }

    // Update P&L from the market snapshot
    const latestSnap = feed.getSnapshot()
    const totalPnl = latestSnap.positions.reduce((acc, pos) => acc + pos.pnl, 0)
    setPnl(Number(totalPnl.toFixed(2)))
  }, [feed, symbol, addLog])

  // ── Start / Stop ──
  const startBot = useCallback(() => {
    if (runningRef.current) return
    if (nodesRef.current.length === 0) {
      addLog('Cannot start — no nodes in strategy', 'error')
      return
    }
    runningRef.current = true
    setStatus('running')
    priceBufferRef.current = []
    prevValuesRef.current.clear()
    cooldownRef.current = 0
    entryPriceRef.current = 0
    addLog('Bot started', 'info')
    // Evaluate every 500ms for a balance of responsiveness and low CPU usage
    intervalRef.current = setInterval(() => {
      if (runningRef.current) evaluateTick()
    }, 500)
  }, [evaluateTick, addLog])

  const stopBot = useCallback(() => {
    if (!runningRef.current) return
    runningRef.current = false
    setStatus('idle')
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
    addLog('Bot stopped', 'info')
  }, [addLog])

  const loadPreset = useCallback((preset: BotPreset) => {
    runningRef.current = false
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
    setStatus('idle')
    const strategy = createPresetGraph(preset)
    setNodes(strategy.nodes)
    setEdges(strategy.edges)
    setSelectedPreset(preset)
    priceBufferRef.current = []
    prevValuesRef.current.clear()
    cooldownRef.current = 0
    entryPriceRef.current = 0
    addLog(`Loaded ${presetLabel(preset)} preset`, 'info')
  }, [addLog])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      runningRef.current = false
      if (intervalRef.current) clearInterval(intervalRef.current)
    }
  }, [])

  useEffect(() => {
    if (nodesRef.current.length > 0 || edgesRef.current.length > 0) return
    const strategy = createPresetGraph('scalper')
    setNodes(strategy.nodes)
    setEdges(strategy.edges)
  }, [])

  const strategy: BotStrategy = { nodes, edges }

  return {
    strategy,
    nodes,
    edges,
    status,
    logs,
    pnl,
    selectedPreset,
    addNode,
    removeNode,
    updateNodeParams,
    moveNode,
    addEdge,
    removeEdge,
    clearAll,
    startBot,
    stopBot,
    loadPreset,
  }
}
