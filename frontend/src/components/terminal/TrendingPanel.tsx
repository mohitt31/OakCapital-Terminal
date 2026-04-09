import { Flame } from 'lucide-react'
import type { TrendingStock } from '../../types/market'
import { monoClass, scrollClass } from './constants'

type Props = {
  stocks: TrendingStock[]
  onSelectSymbol?: (symbol: string) => void
}

export function TrendingPanel({ stocks, onSelectSymbol }: Props) {
  return (
    <section className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden rounded border border-[#2B2F36] bg-[#10141A]">
      <header className="border-b border-[#2B2F36] px-2 py-1 text-[11px] font-semibold tracking-wide text-[#AAB0B6]">
        <div className="flex items-center gap-1">
          <Flame size={14} />
          TRENDING STOCKS
        </div>
      </header>
      <div className="grid grid-cols-3 gap-1 border-b border-[#2B2F36] px-2 py-1 text-[10px] text-[#AAB0B6]">
        <span>Symbol</span>
        <span className="text-right">Price</span>
        <span className="text-right">Change</span>
      </div>
      <div className={`flex-1 space-y-px overflow-y-auto px-1 py-1 ${scrollClass}`}>
        {stocks.map((stock) => (
          <button
            key={stock.symbol}
            className="grid w-full grid-cols-3 rounded bg-[#0B0E11] px-1 py-[3px] text-[10px] hover:bg-[#1C2128]/60"
            onClick={() => onSelectSymbol?.(stock.symbol)}
          >
            <span className="text-left font-medium text-[#D9DEE3]">{stock.symbol}</span>
            <span className={`text-right ${monoClass} text-[#AAB0B6]`}>{stock.price.toFixed(2)}</span>
            <span className={`text-right ${monoClass} ${stock.changePct >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>
              {stock.changePct >= 0 ? '+' : ''}
              {stock.changePct.toFixed(2)}%
            </span>
          </button>
        ))}
      </div>
    </section>
  )
}
