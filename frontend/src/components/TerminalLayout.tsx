import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ComponentType } from 'react'
import { Briefcase, List, Eye, PieChart, Wallet, X, Bot, Crosshair, SlidersHorizontal, TrendingUp, House } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useMockMarket } from '../hooks/useMockMarket'
import { useMarketData } from '../hooks/useMarketData'
import { useTradeControls } from '../hooks/useTradeControls'
import { useAlphaBot } from '../hooks/useAlphaBot'
import type { BookLevel, CandlePoint } from '../types/market'
import { ChartPanel } from './terminal/ChartPanel'
import { ControlPanel } from './terminal/ControlPanel'
import { OrderBookPanel } from './terminal/OrderBookPanel'
import { PortfolioPanel } from './terminal/PortfolioPanel'
import { TrendingPanel } from './terminal/TrendingPanel'
import { BotPanel } from './terminal/BotPanel'
import { AlphaBotEditor } from './alphaBot/AlphaBotEditor'

type DragAxis = 'x' | 'y'
type DragTarget = 'left' | 'rightDrawer'

const MIN_PANEL_PX = 140

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value))
}

function normalizeSymbol(symbol: string) {
  return symbol.toUpperCase().replace(/[^A-Z0-9]/g, '')
}

function priceForSymbol(symbol: string, trending: { symbol: string; price: number }[], fallback: number) {
  const stock = trending.find((row) => row.symbol === symbol)
  return stock?.price ?? fallback
}

function tickSizeForPrice(price: number) {
  if (price < 100) return 0.01
  if (price < 1000) return 0.05
  if (price < 5000) return 0.1
  return 0.5
}

function buildBookFromTemplate(levels: BookLevel[], centerPrice: number, side: 'bid' | 'ask'): BookLevel[] {
  const tick = tickSizeForPrice(centerPrice)
  let cumulative = 0
  return levels.map((level, idx) => {
    const price = side === 'bid'
      ? Number((centerPrice - tick * (idx + 1)).toFixed(2))
      : Number((centerPrice + tick * (idx + 1)).toFixed(2))
    cumulative += level.quantity
    return {
      ...level,
      price,
      cumulative: Number(cumulative.toFixed(3)),
    }
  })
}

function buildCandlesFromTicks(ticks: { time: number; price: number }[], bucketSeconds: number): CandlePoint[] {
  if (ticks.length === 0) return []
  const output: CandlePoint[] = []
  let current: CandlePoint | null = null
  for (const tick of ticks) {
    const bucketTime = Math.floor(tick.time / bucketSeconds) * bucketSeconds
    if (!current || current.time !== bucketTime) {
      if (current) output.push(current)
      current = { time: bucketTime, open: tick.price, high: tick.price, low: tick.price, close: tick.price }
      continue
    }
    current.high = Math.max(current.high, tick.price)
    current.low = Math.min(current.low, tick.price)
    current.close = tick.price
  }
  if (current) output.push(current)
  return output.slice(-600)
}

type NavRailButtonProps = {
  icon: ComponentType<{ className?: string; strokeWidth?: number }>
  label: string
  isActive: boolean
  onClick: () => void
  className?: string
}

function NavRailButton({ icon: Icon, label, isActive, onClick, className = '' }: NavRailButtonProps) {
  return (
    <button
      title={label}
      onClick={onClick}
      className={`relative flex w-full items-center justify-center py-2 transition-colors ${isActive ? 'text-[#00C076] bg-[#1E2329]' : 'text-[#848E9C] hover:text-[#D9DEE3] hover:bg-[#1E2329]/50'
        } ${className}`}
    >
      {isActive && (
        <div className="absolute left-0 top-1/2 w-[3px] h-3/5 -translate-y-1/2 bg-[#00C076] rounded-r-md" />
      )}
      <Icon className="h-[18px] w-[18px]" strokeWidth={1.6} />
    </button>
  )
}


export default function TerminalLayout() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const feed = useMockMarket()
  const { snapshot: market, telemetry } = useMarketData(feed, 100)
  const assetOptions = useMemo(
    () => market.trendingStocks.map((row) => row.symbol),
    [market.trendingStocks],
  )
  const [selectedSymbol, setSelectedSymbol] = useState(() => {
    const routeSymbol = searchParams.get('symbol')
    if (!routeSymbol) return assetOptions[0] || 'RELIANCE'
    const normalizedRaw = normalizeSymbol(routeSymbol)
    const resolved = assetOptions.find((symbol) => normalizeSymbol(symbol) === normalizedRaw)
    return resolved || assetOptions[0] || 'RELIANCE'
  })
  const [secondarySymbol, setSecondarySymbol] = useState(() => assetOptions[1] || assetOptions[0] || 'TCS')
  const [symbolTicksMap, setSymbolTicksMap] = useState<Record<string, { time: number; price: number }[]>>({})
  
  useEffect(() => {
    if (assetOptions.length === 0) return
    if (!assetOptions.includes(selectedSymbol)) setSelectedSymbol(assetOptions[0])
  }, [assetOptions, selectedSymbol])

  useEffect(() => {
    const routeSymbol = searchParams.get('symbol')
    if (!routeSymbol) return
    if (assetOptions.length === 0) return
    const normalizedRaw = normalizeSymbol(routeSymbol)
    const resolved = assetOptions.find((symbol) => normalizeSymbol(symbol) === normalizedRaw)
    if (resolved && resolved !== selectedSymbol) {
      setSelectedSymbol(resolved)
    }
  }, [searchParams, assetOptions, selectedSymbol])
  useEffect(() => {
    const nowSec = Math.floor(Date.now() / 1000)
    setSymbolTicksMap((prev) => {
      const next: Record<string, { time: number; price: number }[]> = { ...prev }
      market.trendingStocks.forEach((stock) => {
        let stream = next[stock.symbol]
        if (!stream) {
          const ratio = market.lastPrice === 0 ? 1 : stock.price / market.lastPrice
          stream = market.candles1s.map((candle) => ({
            time: candle.time,
            price: Number((candle.close * ratio).toFixed(2)),
          }))
        } else {
          stream = [...stream]
        }
        const last = stream[stream.length - 1]
        if (!last || last.time !== nowSec || last.price !== stock.price) {
          stream.push({ time: nowSec, price: stock.price })
          if (stream.length > 1800) stream.splice(0, stream.length - 1800)
        }
        next[stock.symbol] = stream
      })
      return next
    })
  }, [market.trendingStocks, market.candles1s, market.lastPrice])

  const marketView = useMemo(() => {
    const selectedPrice = priceForSymbol(selectedSymbol, market.trendingStocks, market.lastPrice)
    let ticks = symbolTicksMap[selectedSymbol] ?? []
    if (ticks.length === 0 && market.candles1s.length > 0) {
      const ratio = market.lastPrice === 0 ? 1 : selectedPrice / market.lastPrice
      ticks = market.candles1s.map((candle) => ({
        time: candle.time,
        price: Number((candle.close * ratio).toFixed(2)),
      }))
    }
    const candles1s = buildCandlesFromTicks(ticks, 1)
    const candles5s = buildCandlesFromTicks(ticks, 5)
    const bids = buildBookFromTemplate(market.bids, selectedPrice, 'bid')
    const asks = buildBookFromTemplate(market.asks, selectedPrice, 'ask')
    const latest = candles1s[candles1s.length - 1]
    const prev = candles1s[candles1s.length - 2]
    const tickDirection: 1 | -1 | 0 = !latest || !prev ? market.tickDirection : latest.close > prev.close ? 1 : latest.close < prev.close ? -1 : 0
    return {
      ...market,
      lastPrice: selectedPrice,
      tickDirection,
      bids,
      asks,
      candles1s,
      candles5s,
    }
  }, [market, selectedSymbol, symbolTicksMap])

  const secondaryMarketView = useMemo(() => {
    const selectedPrice = priceForSymbol(secondarySymbol, market.trendingStocks, market.lastPrice)
    let ticks = symbolTicksMap[secondarySymbol] ?? []
    if (ticks.length === 0 && market.candles1s.length > 0) {
      const ratio = market.lastPrice === 0 ? 1 : selectedPrice / market.lastPrice
      ticks = market.candles1s.map((candle) => ({
        time: candle.time,
        price: Number((candle.close * ratio).toFixed(2)),
      }))
    }
    const candles1s = buildCandlesFromTicks(ticks, 1)
    const candles5s = buildCandlesFromTicks(ticks, 5)
    return { candles1s, candles5s }
  }, [secondarySymbol, market.trendingStocks, market.lastPrice, symbolTicksMap, market.candles1s])

  const controls = useTradeControls(marketView.lastPrice, selectedSymbol, (request) => feed.executeOrder(request))
  const controlsRef = useRef(controls)
  const bot = useAlphaBot(feed, selectedSymbol)
  const [showBotEditor, setShowBotEditor] = useState(false)

  useEffect(() => {
    controlsRef.current = controls
  }, [controls])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      const tag = target?.tagName?.toLowerCase()
      const isTyping =
        tag === 'input' || tag === 'textarea' || target?.isContentEditable === true
      if (isTyping) return
      const key = event.key.toLowerCase()
      if (key === 'l') { event.preventDefault(); controlsRef.current.setLong() }
      else if (key === 'k') { event.preventDefault(); controlsRef.current.setShort() }
      else if (key === 'q') { event.preventDefault(); controlsRef.current.quickBuy() }
      else if (key === 'e') { event.preventDefault(); controlsRef.current.quickSell() }
      else if (key === 'm') { event.preventDefault(); controlsRef.current.setOrderType('market') }
      else if (key === 'n') { event.preventDefault(); controlsRef.current.setOrderType('limit') }
      else if (key === 'b') { event.preventDefault(); controlsRef.current.execute('buy') }
      else if (key === 's') { event.preventDefault(); controlsRef.current.execute('sell') }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  const containerRef = useRef<HTMLDivElement | null>(null)
  const dragRef = useRef<{ target: DragTarget; axis: DragAxis; startPos: number; startValue: number } | null>(null)

  const [leftWidth, setLeftWidth] = useState(15)
  const [rightDrawerWidth, setRightDrawerWidth] = useState(320)
  const [rightNavTab, setRightNavTab] = useState<string | null>('controls')

  const toggleRightNav = (tab: string) => {
    setRightNavTab((prev) => (prev === tab ? null : tab))
  }

  const onPointerDown = useCallback((target: DragTarget, axis: DragAxis, event: React.PointerEvent) => {
    event.preventDefault()
    const startPos = axis === 'x' ? event.clientX : event.clientY
    const startValue = target === 'left' ? leftWidth : rightDrawerWidth
    dragRef.current = { target, axis, startPos, startValue }
    document.body.style.cursor = axis === 'x' ? 'col-resize' : 'row-resize'
    document.body.style.userSelect = 'none'
  }, [leftWidth, rightDrawerWidth])

  useEffect(() => {
    const onPointerMove = (event: PointerEvent) => {
      const drag = dragRef.current
      if (!drag) return
      const container = containerRef.current
      if (!container) return
      const rect = container.getBoundingClientRect()
      const delta = (drag.axis === 'x' ? event.clientX : event.clientY) - drag.startPos
      const totalPx = drag.axis === 'x' ? rect.width : rect.height
      const deltaPct = (delta / totalPx) * 100
      const minPct = (MIN_PANEL_PX / totalPx) * 100

      if (drag.target === 'left') {
        setLeftWidth(clamp(drag.startValue + deltaPct, minPct, 35))
      } else if (drag.target === 'rightDrawer') {
        const minDrawerPx = 280
        const maxDrawerPx = Math.max(360, rect.width * 0.58)
        setRightDrawerWidth(clamp(drag.startValue - delta, minDrawerPx, maxDrawerPx))
      }
    }

    const onPointerUp = () => {
      dragRef.current = null
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }

    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', onPointerUp)
    return () => {
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('pointerup', onPointerUp)
    }
  }, [])

  const centerWidth = 100 - leftWidth

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#0B0E11] text-[#D9DEE3]">
      <main className="flex-1 overflow-hidden p-1.5 flex flex-col min-w-0" style={{ paddingRight: rightNavTab ? 0 : '1.5px' }}>
        <div ref={containerRef} className="flex h-full flex-col gap-0 border border-[#2B2F36] rounded bg-[#0E1114]">
          <div className="flex min-h-0 flex-1">

            <div className="min-h-0 min-w-0 overflow-hidden" style={{ width: `${leftWidth}%` }}>
              <OrderBookPanel symbol={selectedSymbol} lastPrice={marketView.lastPrice} bids={marketView.bids} asks={marketView.asks} />
            </div>

            <div
              className="group relative z-10 flex w-[3px] shrink-0 cursor-col-resize items-center justify-center"
              onPointerDown={(e) => onPointerDown('left', 'x', e)}
            >
              <div className="h-full w-px bg-transparent transition-colors group-hover:bg-[#00C076]/35" />
            </div>

            <div className="min-h-0 min-w-0 overflow-hidden" style={{ width: `${centerWidth}%` }}>
              <ChartPanel
                lastPrice={marketView.lastPrice}
                tickDirection={marketView.tickDirection}
                candles1s={marketView.candles1s}
                candles5s={marketView.candles5s}
                symbol={selectedSymbol}
                secondarySymbol={secondarySymbol}
                secondaryCandles1s={secondaryMarketView.candles1s}
                secondaryCandles5s={secondaryMarketView.candles5s}
                onSelectSecondaryAsset={setSecondarySymbol}
                assets={assetOptions}
                onSelectAsset={setSelectedSymbol}
                onQuickTrade={(action, quantity, orderType, limitPrice) =>
                  feed.executeOrder({
                    asset: selectedSymbol,
                    action,
                    direction: action === 'buy' ? 'long' : 'short',
                    quantity,
                    orderType,
                    limitPrice: orderType === 'limit' ? limitPrice : marketView.lastPrice,
                    timestamp: Date.now(),
                  })
                }
              />
            </div>
          </div>
        </div>
      </main>

      {/* Right Drawer */}
      {rightNavTab && (
        <div className="relative shrink-0 border-l border-[#2B2F36] bg-[#0E1115] flex flex-col" style={{ width: `${rightDrawerWidth}px` }}>
          {/* Resize Overlay */}
          <div
            className="absolute top-0 bottom-0 left-0 z-50 w-[4px] cursor-col-resize bg-transparent transition-colors hover:bg-[#00C076]/40"
            onPointerDown={(e) => onPointerDown('rightDrawer', 'x', e)}
          />
          <div className="flex items-center justify-between border-b border-[#2B2F36] p-4">
            <h2 className="text-sm font-semibold capitalize tracking-wide text-[#EAECEF]">{rightNavTab}</h2>
            <button onClick={() => setRightNavTab(null)} className="text-[#848E9C] hover:text-[#D9DEE3] transition-colors">
              <X className="h-5 w-5" />
            </button>
          </div>
          {rightNavTab === 'trending' ? (
            <div className="flex-1 overflow-hidden flex flex-col">
              <TrendingPanel stocks={market.trendingStocks} onSelectSymbol={setSelectedSymbol} />
            </div>
          ) : rightNavTab === 'portfolio' ? (
            <div className="flex-1 overflow-hidden flex flex-col">
              <PortfolioPanel positions={marketView.positions} cashBalance={marketView.cashBalance} />
            </div>
          ) : rightNavTab === 'controls' ? (
            <div className="flex-1 overflow-hidden flex flex-col">
              <ControlPanel
                symbol={selectedSymbol}
                lastPrice={marketView.lastPrice}
                tickDirection={marketView.tickDirection}
                direction={controls.direction}
                orderType={controls.orderType}
                quantityText={controls.quantityText}
                priceText={controls.priceText}
                intent={controls.intent}
                onSetLong={controls.setLong}
                onSetShort={controls.setShort}
                onSetOrderType={controls.setOrderType}
                onQuantityChange={controls.setQuantityText}
                onPriceChange={controls.setPriceText}
                onQuickBuy={controls.quickBuy}
                onQuickSell={controls.quickSell}
                onBuy={() => controls.execute('buy')}
                onSell={() => controls.execute('sell')}
                telemetry={telemetry}
                fills={marketView.fills.filter((fill) => fill.asset === selectedSymbol)}
              />
            </div>
          ) : rightNavTab === 'bot' ? (
            <div className="flex-1 overflow-hidden flex flex-col">
              <BotPanel
                symbol={selectedSymbol}
                status={bot.status}
                pnl={bot.pnl}
                logs={bot.logs}
                nodeCount={bot.nodes.length}
                edgeCount={bot.edges.length}
                selectedPreset={bot.selectedPreset}
                onSelectPreset={bot.loadPreset}
                onStart={bot.startBot}
                onStop={bot.stopBot}
                onOpenEditor={() => setShowBotEditor(true)}
                onClear={bot.clearAll}
              />
            </div>
          ) : rightNavTab === 'orders' ? (
            <div className="flex-1 overflow-y-auto p-3">
              <div className="mb-2 text-[11px] font-semibold tracking-wide text-[#AAB0B6]">RECENT ORDERS</div>
              <div className="space-y-1">
                {(marketView.fills.length > 0
                  ? marketView.fills
                  : [
                      { id: -1, asset: selectedSymbol, action: 'buy' as const, quantity: 2.5, price: marketView.lastPrice, timestamp: 0 },
                      { id: -2, asset: selectedSymbol, action: 'sell' as const, quantity: 1.2, price: marketView.lastPrice * 1.002, timestamp: 0 },
                    ]
                ).slice(0, 20).map((fill) => (
                  <div key={fill.id} className="grid grid-cols-4 rounded border border-[#2B2F36] bg-[#11161E] px-2 py-1 text-[10px]">
                    <span className="truncate text-[#AAB0B6]">{fill.asset}</span>
                    <span className={fill.action === 'buy' ? 'text-[#00C076]' : 'text-[#FF3B30]'}>{fill.action.toUpperCase()}</span>
                    <span className="text-right font-mono text-[#D9DEE3]">{fill.quantity.toFixed(3)}</span>
                    <span className="text-right font-mono text-[#D9DEE3]">{fill.price.toFixed(2)}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : rightNavTab === 'positions' ? (
            <div className="flex-1 overflow-y-auto p-3">
              <div className="mb-2 text-[11px] font-semibold tracking-wide text-[#AAB0B6]">OPEN POSITIONS</div>
              <div className="space-y-1">
                {(marketView.positions.length > 0
                  ? marketView.positions
                  : [
                      { asset: selectedSymbol, quantity: 5, entryPrice: marketView.lastPrice * 0.99, markPrice: marketView.lastPrice, pnl: marketView.lastPrice * 0.05 },
                    ]
                ).map((pos) => (
                  <div key={pos.asset} className="grid grid-cols-4 rounded border border-[#2B2F36] bg-[#11161E] px-2 py-1 text-[10px]">
                    <span className="truncate text-[#AAB0B6]">{pos.asset}</span>
                    <span className={`text-right font-mono ${pos.quantity >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{pos.quantity.toFixed(3)}</span>
                    <span className="text-right font-mono text-[#D9DEE3]">{pos.markPrice.toFixed(2)}</span>
                    <span className={`text-right font-mono ${pos.pnl >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{pos.pnl.toFixed(2)}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : rightNavTab === 'watchlist' ? (
            <div className="flex-1 overflow-y-auto p-3">
              <div className="mb-2 text-[11px] font-semibold tracking-wide text-[#AAB0B6]">WATCHLIST</div>
              <div className="space-y-1">
                {(market.trendingStocks.length > 0
                  ? market.trendingStocks
                  : [
                      { symbol: 'RELIANCE', price: 2970, changePct: 0.42, volume: 1340000 },
                      { symbol: 'TCS', price: 4185, changePct: -0.28, volume: 820000 },
                    ]
                ).slice(0, 16).map((row) => (
                  <button key={row.symbol} onClick={() => setSelectedSymbol(row.symbol)} className="grid w-full grid-cols-3 rounded border border-[#2B2F36] bg-[#11161E] px-2 py-1 text-[10px] hover:bg-[#1A212B]">
                    <span className="truncate text-left text-[#AAB0B6]">{row.symbol}</span>
                    <span className="text-right font-mono text-[#D9DEE3]">{row.price.toFixed(2)}</span>
                    <span className={`text-right font-mono ${row.changePct >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{row.changePct.toFixed(2)}%</span>
                  </button>
                ))}
              </div>
            </div>
          ) : rightNavTab === 'holdings' ? (
            <div className="flex-1 overflow-y-auto p-3">
              <div className="mb-2 text-[11px] font-semibold tracking-wide text-[#AAB0B6]">HOLDINGS</div>
              <div className="space-y-1">
                {(marketView.positions.filter((p) => p.quantity > 0).length > 0
                  ? marketView.positions.filter((p) => p.quantity > 0)
                  : [
                      { asset: selectedSymbol, quantity: 10, entryPrice: marketView.lastPrice * 0.96, markPrice: marketView.lastPrice, pnl: marketView.lastPrice * 0.4 },
                    ]
                ).map((pos) => (
                  <div key={pos.asset} className="grid grid-cols-4 rounded border border-[#2B2F36] bg-[#11161E] px-2 py-1 text-[10px]">
                    <span className="truncate text-[#AAB0B6]">{pos.asset}</span>
                    <span className="text-right font-mono text-[#D9DEE3]">{pos.quantity.toFixed(3)}</span>
                    <span className="text-right font-mono text-[#D9DEE3]">{pos.markPrice.toFixed(2)}</span>
                    <span className={`text-right font-mono ${pos.pnl >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{pos.pnl.toFixed(2)}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : rightNavTab === 'balance' ? (
            <div className="flex-1 overflow-y-auto p-3">
              <div className="mb-2 text-[11px] font-semibold tracking-wide text-[#AAB0B6]">BALANCE</div>
              <div className="space-y-2">
                <div className="rounded border border-[#2B2F36] bg-[#11161E] px-3 py-2">
                  <div className="text-[10px] text-[#6D7480]">Cash</div>
                  <div className="text-[13px] font-mono text-[#D9DEE3]">{marketView.cashBalance.toFixed(2)}</div>
                </div>
                <div className="rounded border border-[#2B2F36] bg-[#11161E] px-3 py-2">
                  <div className="text-[10px] text-[#6D7480]">Equity</div>
                  <div className="text-[13px] font-mono text-[#D9DEE3]">{(marketView.cashBalance + marketView.positions.reduce((acc, p) => acc + p.markPrice * p.quantity, 0)).toFixed(2)}</div>
                </div>
                <div className="rounded border border-[#2B2F36] bg-[#11161E] px-3 py-2">
                  <div className="text-[10px] text-[#6D7480]">Margin Used</div>
                  <div className="text-[13px] font-mono text-[#AAB0B6]">{Math.max(0, marketView.positions.reduce((acc, p) => acc + Math.abs(p.quantity * p.markPrice), 0) * 0.1).toFixed(2)}</div>
                </div>
              </div>
            </div>
          ) : (
            <div className="flex-1 overflow-y-auto p-4 flex flex-col items-center justify-center text-[#848E9C]">
              <div className="mb-6 rounded-2xl bg-[#161A1E] p-8 border border-[#2B2F36] shadow-sm">
                {rightNavTab === 'positions' && <Crosshair className="h-10 w-10 text-[#00C076]" strokeWidth={1} />}
                {rightNavTab === 'orders' && <List className="h-10 w-10 text-[#00C076]" strokeWidth={1} />}
                {rightNavTab === 'watchlist' && <Eye className="h-10 w-10 text-[#00C076]" strokeWidth={1} />}
                {rightNavTab === 'holdings' && <PieChart className="h-10 w-10 text-[#00C076]" strokeWidth={1} />}
                {rightNavTab === 'balance' && <Wallet className="h-10 w-10 text-[#00C076]" strokeWidth={1} />}
              </div>
              <p className="text-sm font-medium">You have no open {rightNavTab}</p>
              <button className="mt-4 px-4 py-1.5 text-xs text-[#00C076] hover:bg-[#00C076]/10 rounded-md transition-colors font-medium">
                Refresh
              </button>
            </div>
          )}
        </div>
      )}

      {/* Right Navigation Rail */}
      <div className="w-[48px] shrink-0 border-l border-[#2B2F36] bg-[#0B0E11] flex flex-col items-center py-3 gap-1.5 z-20 overflow-y-auto">
        <NavRailButton icon={House} label="Home" isActive={false} onClick={() => navigate('/')} />
        <NavRailButton icon={SlidersHorizontal} label="Controls" isActive={rightNavTab === 'controls'} onClick={() => toggleRightNav('controls')} />
        <NavRailButton icon={TrendingUp} label="Trending" isActive={rightNavTab === 'trending'} onClick={() => toggleRightNav('trending')} />
        <NavRailButton icon={Briefcase} label="Portfolio" isActive={rightNavTab === 'portfolio'} onClick={() => toggleRightNav('portfolio')} />
        <NavRailButton icon={Bot} label="Bot" isActive={rightNavTab === 'bot'} onClick={() => toggleRightNav('bot')} />
        <NavRailButton icon={Crosshair} label="Positions" isActive={rightNavTab === 'positions'} onClick={() => toggleRightNav('positions')} />
        <NavRailButton icon={List} label="Orders" isActive={rightNavTab === 'orders'} onClick={() => toggleRightNav('orders')} />
        <NavRailButton icon={Eye} label="Watchlist" isActive={rightNavTab === 'watchlist'} onClick={() => toggleRightNav('watchlist')} />
        <NavRailButton icon={PieChart} label="Holdings" isActive={rightNavTab === 'holdings'} onClick={() => toggleRightNav('holdings')} />
        <NavRailButton icon={Wallet} label="Balance" isActive={rightNavTab === 'balance'} onClick={() => toggleRightNav('balance')} />

        <div className="mt-auto flex w-full flex-col gap-2">
          <NavRailButton icon={Bot} label="Bot Editor" isActive={showBotEditor} onClick={() => setShowBotEditor(true)} />
        </div>
      </div>
      {showBotEditor && (
        <AlphaBotEditor
          nodes={bot.nodes}
          edges={bot.edges}
          status={bot.status}
          logs={bot.logs}
          pnl={bot.pnl}
          onAddNode={bot.addNode}
          onRemoveNode={bot.removeNode}
          onUpdateParams={bot.updateNodeParams}
          onMoveNode={bot.moveNode}
          onAddEdge={bot.addEdge}
          onClear={bot.clearAll}
          onStart={bot.startBot}
          onStop={bot.stopBot}
          onClose={() => setShowBotEditor(false)}
        />
      )}
    </div>
  )
}
