import type { PortfolioPosition } from '../../types/market'
import { monoClass, scrollClass } from './constants'

type Props = {
  positions: PortfolioPosition[]
  cashBalance: number
}

export function PortfolioPanel({ positions, cashBalance }: Props) {
  const holdingsValue = positions.reduce((sum, position) => sum + Math.abs(position.markPrice * position.quantity), 0)
  const livePnl = positions.reduce((sum, position) => sum + position.pnl, 0)
  const netExposure = positions.reduce((sum, position) => sum + position.markPrice * position.quantity, 0)
  const equity = cashBalance + netExposure
  return (
    <section className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden text-[#D9DEE3]">
      <div className="grid grid-cols-2 gap-2 p-4 text-[11px]">
        <div className="rounded-lg border border-[#2B2F36] bg-[#161a1e] p-3 text-center transition-colors hover:bg-[#1E2329]">
          <div className="text-[#848E9C] mb-1">Cash</div>
          <div className={`text-sm font-semibold ${monoClass}`}>{cashBalance.toFixed(2)}</div>
        </div>
        <div className="rounded-lg border border-[#2B2F36] bg-[#161a1e] p-3 text-center transition-colors hover:bg-[#1E2329]">
          <div className="text-[#848E9C] mb-1">Holdings</div>
          <div className={`text-sm font-semibold ${monoClass}`}>{holdingsValue.toFixed(2)}</div>
        </div>
        <div className="rounded-lg border border-[#2B2F36] bg-[#161a1e] p-3 text-center transition-colors hover:bg-[#1E2329]">
          <div className="text-[#848E9C] mb-1">Live P&L</div>
          <div className={`text-sm font-semibold ${monoClass} ${livePnl >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{livePnl.toFixed(2)}</div>
        </div>
        <div className="rounded-lg border border-[#2B2F36] bg-[#161a1e] p-3 text-center transition-colors hover:bg-[#1E2329]">
          <div className="text-[#848E9C] mb-1">Equity</div>
          <div className={`text-sm font-semibold ${monoClass} ${equity >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{equity.toFixed(2)}</div>
        </div>
      </div>

      <div className="px-4 pb-2 text-[12px] font-semibold text-[#EAECEF] mt-2 border-t border-[#2B2F36] pt-4">Open Positions</div>

      <div className={`flex-1 space-y-2 overflow-y-auto px-4 pb-4 ${scrollClass}`}>
        {positions.length === 0 ? (
          <div className="py-8 text-center text-xs text-[#848E9C]">No open positions.</div>
        ) : positions.map((position) => (
          <div key={position.asset} className="flex flex-col gap-1 rounded-lg border border-[#2B2F36] bg-[#161a1e] p-3 text-[11px] transition-colors hover:bg-[#1E2329]">
            <div className="flex justify-between font-semibold">
              <span className="text-[#EAECEF]">{position.asset}</span>
              <span className={`${position.quantity >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>
                {position.quantity >= 0 ? 'LONG' : 'SHORT'} <span className="opacity-70 ml-1 font-normal">({position.quantity.toFixed(3)})</span>
              </span>
            </div>
            <div className="flex justify-between text-[#848E9C] mt-2 border-t border-[#2B2F36] pt-2">
              <div className="flex flex-col">
                <span className="text-[9px]">Entry</span>
                <span className={monoClass}>{position.entryPrice.toFixed(2)}</span>
              </div>
              <div className="flex flex-col text-right">
                <span className="text-[9px]">Live P&L</span>
                <span className={`text-[12px] font-medium ${monoClass} ${position.pnl >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>
                  {position.pnl >= 0 ? '+' : ''}{position.pnl.toFixed(2)}
                </span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

