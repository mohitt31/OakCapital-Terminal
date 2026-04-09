import { Gauge } from 'lucide-react'
import type { BookLevel } from '../../types/market'
import { monoClass, scrollClass } from './constants'

type Props = {
  symbol: string
  lastPrice: number
  bids: BookLevel[]
  asks: BookLevel[]
}

export function OrderBookPanel({ symbol, lastPrice, bids, asks }: Props) {
  const maxBidDepth = bids[bids.length - 1]?.cumulative ?? 1
  const maxAskDepth = asks[asks.length - 1]?.cumulative ?? 1
  const now = Date.now()

  return (
    <section className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden border-0 bg-[#10141A]">
      <header className="flex items-center justify-between border-b border-[#2B2F36] px-2 py-1">
        <div className="flex items-center gap-1 text-[11px] font-semibold tracking-wide text-[#AAB0B6]">
          <Gauge size={14} />
          ORDER BOOK · {symbol}
        </div>
        <div className={`text-[12px] ${monoClass}`}>{lastPrice.toFixed(2)}</div>
      </header>
      <div className="grid flex-1 grid-rows-2 overflow-hidden text-[11px]">
        <div className="overflow-hidden border-b border-[#2B2F36]">
          <div className="grid grid-cols-[1fr_0.9fr_0.9fr] px-2 py-1 text-[#AAB0B6]">
            <span>Price</span>
            <span className="text-right">Qty</span>
            <span className="text-right">Cum</span>
          </div>
          <div className={`no-scrollbar h-full space-y-px overflow-y-auto px-1 pb-1 ${scrollClass}`}>
            {asks
              .slice()
              .reverse()
              .map((level, idx) => {
                const depthWidth = `${Math.min(100, (level.cumulative / maxAskDepth) * 100)}%`
                const flashing = level.flashUntil > now
                return (
                  <div key={`ask-${idx}-${level.price.toFixed(2)}`} className="relative grid grid-cols-[1fr_0.9fr_0.9fr] overflow-hidden rounded px-1 py-[2px] hover:bg-[#1C2128]/35">
                    <span
                      className={`absolute inset-y-0 right-0 bg-[#FF3B30]/20 ${flashing ? 'bg-[#FF3B30]/35' : ''}`}
                      style={{ width: depthWidth }}
                    />
                  <span className={`relative truncate ${monoClass} text-[#FF6A61]`}>{level.price.toFixed(2)}</span>
                  <span className={`relative truncate text-right ${monoClass}`}>{level.quantity.toFixed(3)}</span>
                  <span className={`relative truncate text-right ${monoClass} text-[#AAB0B6]`}>{level.cumulative.toFixed(3)}</span>
                  </div>
                )
              })}
          </div>
        </div>
        <div className="overflow-hidden">
          <div className="grid grid-cols-[1fr_0.9fr_0.9fr] px-2 py-1 text-[#AAB0B6]">
            <span>Price</span>
            <span className="text-right">Qty</span>
            <span className="text-right">Cum</span>
          </div>
          <div className={`no-scrollbar h-full space-y-px overflow-y-auto px-1 pb-1 ${scrollClass}`}>
            {bids.map((level, idx) => {
              const depthWidth = `${Math.min(100, (level.cumulative / maxBidDepth) * 100)}%`
              const flashing = level.flashUntil > now
              return (
                <div key={`bid-${idx}-${level.price.toFixed(2)}`} className="relative grid grid-cols-[1fr_0.9fr_0.9fr] overflow-hidden rounded px-1 py-[2px] hover:bg-[#1C2128]/35">
                  <span
                    className={`absolute inset-y-0 right-0 bg-[#00C076]/20 ${flashing ? 'bg-[#00C076]/35' : ''}`}
                    style={{ width: depthWidth }}
                  />
                  <span className={`relative truncate ${monoClass} text-[#35D393]`}>{level.price.toFixed(2)}</span>
                  <span className={`relative truncate text-right ${monoClass}`}>{level.quantity.toFixed(3)}</span>
                  <span className={`relative truncate text-right ${monoClass} text-[#AAB0B6]`}>{level.cumulative.toFixed(3)}</span>
                </div>
              )
            })}
          </div>
        </div>
      </div>
    </section>
  )
}
