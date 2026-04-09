import type { TradeIntent } from '../../hooks/useTradeControls'
import type { MarketTelemetry } from '../../hooks/useMarketData'
import type { TradeFill } from '../../types/market'
import { monoClass, scrollClass } from './constants'

type Props = {
  symbol: string
  lastPrice: number
  tickDirection: 1 | -1 | 0
  direction: 'long' | 'short'
  orderType: 'limit' | 'market'
  quantityText: string
  priceText: string
  intent: TradeIntent
  onSetLong: () => void
  onSetShort: () => void
  onSetOrderType: (value: 'limit' | 'market') => void
  onQuantityChange: (value: string) => void
  onPriceChange: (value: string) => void
  onQuickBuy: () => void
  onQuickSell: () => void
  onBuy: () => void
  onSell: () => void
  telemetry: MarketTelemetry
  fills: TradeFill[]
}

export function ControlPanel({
  symbol,
  lastPrice,
  tickDirection,
  orderType,
  quantityText,
  priceText,
  intent,
  onSetOrderType,
  onQuantityChange,
  onPriceChange,
  onBuy,
  onSell,
  fills,
}: Props) {
  return (
    <section className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden text-[#D9DEE3]">
      <header className="shrink-0 border-b border-[#2B2F36] px-2 py-1 text-[11px] font-semibold tracking-wide text-[#AAB0B6]">
        ORDERS · {symbol}
      </header>
      <div className={`flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-4 ${scrollClass}`}>
        <div className="grid shrink-0 grid-cols-2 gap-2 text-[11px] font-semibold">
          <button
            className={`rounded border px-1.5 py-1.5 ${orderType === 'limit' ? 'border-[#60A5FA] bg-[#60A5FA]/15 text-[#60A5FA]' : 'border-[#2B2F36] bg-[#161a1e] text-[#D9DEE3] hover:bg-[#1E2329]'}`}
            onClick={() => onSetOrderType('limit')}
          >
            LIMIT
          </button>
          <button
            className={`rounded border px-1.5 py-1.5 ${orderType === 'market' ? 'border-[#F59E0B] bg-[#F59E0B]/15 text-[#F59E0B]' : 'border-[#2B2F36] bg-[#161a1e] text-[#D9DEE3] hover:bg-[#1E2329]'}`}
            onClick={() => onSetOrderType('market')}
          >
            MARKET
          </button>
        </div>
        <div className="shrink-0 rounded border border-[#2B2F36] bg-[#0B0E11] px-2 py-1">
          <div className="text-[10px] text-[#AAB0B6]">Market Price</div>
          <div className={`text-[14px] leading-tight ${monoClass} ${tickDirection >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>
            {lastPrice.toFixed(2)}
          </div>
        </div>
        <div className="grid shrink-0 grid-cols-2 gap-1">
          <label className="text-[10px] text-[#AAB0B6]">
            QTY
            <input
              className={`mt-0.5 w-full rounded border border-[#2B2F36] bg-[#0B0E11] px-1.5 py-0.5 text-[11px] text-[#D9DEE3] ${monoClass}`}
              value={quantityText}
              onChange={(e) => onQuantityChange(e.target.value)}
            />
          </label>
          <label className="text-[10px] text-[#AAB0B6]">
            PRICE
            <input
              className={`mt-0.5 w-full rounded border border-[#2B2F36] bg-[#0B0E11] px-1.5 py-0.5 text-[11px] text-[#D9DEE3] disabled:cursor-not-allowed disabled:opacity-50 ${monoClass}`}
              value={priceText}
              disabled={orderType === 'market'}
              onChange={(e) => onPriceChange(e.target.value)}
            />
          </label>
        </div>
        <div className="shrink-0 rounded border border-[#2B2F36] bg-[#0B0E11] px-2 py-0.5 text-[10px] text-[#AAB0B6]">
          {intent.sideLabel}
        </div>
        <div className="grid shrink-0 grid-cols-2 gap-1">
          <button className="rounded bg-[#00C076] px-2 py-1.5 text-[11px] font-bold text-black disabled:opacity-40" disabled={!intent.canSubmit} onClick={onBuy}>
            BUY
          </button>
          <button className="rounded bg-[#FF3B30] px-2 py-1.5 text-[11px] font-bold text-white disabled:opacity-40" disabled={!intent.canSubmit} onClick={onSell}>
            SELL
          </button>
        </div>
        <div className="shrink-0 rounded-lg border border-[#2B2F36] bg-[#161a1e] p-2">
          <div className="mb-0.5 text-[10px] font-semibold tracking-wide text-[#AAB0B6]">RECENT FILLS</div>
          <div className="space-y-px mt-2">
            {fills.length === 0 ? (
              <div className="text-[10px] text-[#6D7480] text-center p-2">No fills yet</div>
            ) : (
              fills.slice(0, 5).map((fill) => (
                <div key={fill.id} className="grid grid-cols-4 rounded bg-[#0b0e11] px-1 py-1 text-[10px]">
                  <span className="truncate text-[#AAB0B6]">{fill.asset}</span>
                  <span className={fill.action === 'buy' ? 'text-[#00C076]' : 'text-[#FF3B30]'}>{fill.action.toUpperCase()}</span>
                  <span className={`text-right ${monoClass}`}>{fill.quantity.toFixed(3)}</span>
                  <span className={`text-right ${monoClass}`}>{fill.price.toFixed(2)}</span>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
