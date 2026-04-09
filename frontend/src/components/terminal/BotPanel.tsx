import { useMemo } from 'react'
import { Activity, Bot, Play, Square, SlidersHorizontal, Trophy, WandSparkles } from 'lucide-react'

type BotPanelProps = {
  symbol: string
  status: 'idle' | 'running' | 'error'
  pnl: number
  logs: { id: number; type: 'info' | 'trade' | 'error'; message: string; time: number }[]
  nodeCount: number
  edgeCount: number
  selectedPreset: 'scalper' | 'meanReversion' | 'breakout'
  onSelectPreset: (preset: 'scalper' | 'meanReversion' | 'breakout') => void
  onStart: () => void
  onStop: () => void
  onOpenEditor: () => void
  onClear: () => void
}

const PRESET_LABELS: Record<'scalper' | 'meanReversion' | 'breakout', string> = {
  scalper: 'Scalper',
  meanReversion: 'Mean Reversion',
  breakout: 'Breakout',
}

export function BotPanel({
  symbol,
  status,
  pnl,
  logs,
  nodeCount,
  edgeCount,
  selectedPreset,
  onSelectPreset,
  onStart,
  onStop,
  onOpenEditor,
  onClear,
}: BotPanelProps) {
  const tradeCount = logs.filter((row) => row.type === 'trade').length
  const leaderboard = useMemo(() => {
    const baseline = [
      { name: 'Neural Falcon', pnl: 1842.2, winRate: 66.2, trades: 41, status: 'running' },
      { name: 'Gamma Pulse', pnl: 1288.4, winRate: 61.4, trades: 37, status: 'running' },
      { name: 'Delta MeanX', pnl: 966.5, winRate: 58.9, trades: 33, status: 'idle' },
      { name: 'Sigma Breaker', pnl: 756.8, winRate: 57.2, trades: 29, status: 'idle' },
    ]
    const yours = {
      name: 'Your Bot',
      pnl,
      winRate: status === 'running' ? 60 + Math.min(16, tradeCount * 0.4) : 57.5,
      trades: tradeCount,
      status,
    }
    return [yours, ...baseline].sort((a, b) => b.pnl - a.pnl)
  }, [pnl, status, tradeCount])

  return (
    <div className="flex h-full flex-col overflow-hidden bg-[#0F1319]">
      <div className="border-b border-[#2B2F36] p-3">
        <div className="flex items-center gap-2 text-[#D9DEE3]">
          <Bot className="h-4 w-4 text-[#F0B90B]" />
          <span className="text-[12px] font-semibold tracking-wide">Alpha Bot Console</span>
        </div>
        <div className="mt-1 text-[11px] text-[#7E8794]">{symbol} strategy workspace</div>
      </div>

      <div className="grid grid-cols-2 gap-2 border-b border-[#2B2F36] p-3">
        <div className="rounded border border-[#2B2F36] bg-[#101720] px-2 py-1.5">
          <div className="text-[10px] text-[#7E8794]">Status</div>
          <div className={`text-[11px] font-semibold ${status === 'running' ? 'text-[#00C076]' : status === 'error' ? 'text-[#FF3B30]' : 'text-[#D9DEE3]'}`}>{status.toUpperCase()}</div>
        </div>
        <div className="rounded border border-[#2B2F36] bg-[#101720] px-2 py-1.5">
          <div className="text-[10px] text-[#7E8794]">P&L</div>
          <div className={`text-[11px] font-semibold ${pnl >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{pnl.toFixed(2)}</div>
        </div>
        <div className="rounded border border-[#2B2F36] bg-[#101720] px-2 py-1.5">
          <div className="text-[10px] text-[#7E8794]">Nodes</div>
          <div className="text-[11px] font-semibold text-[#D9DEE3]">{nodeCount}</div>
        </div>
        <div className="rounded border border-[#2B2F36] bg-[#101720] px-2 py-1.5">
          <div className="text-[10px] text-[#7E8794]">Edges</div>
          <div className="text-[11px] font-semibold text-[#D9DEE3]">{edgeCount}</div>
        </div>
      </div>

      <div className="border-b border-[#2B2F36] p-3">
        <div className="mb-2 flex items-center gap-1.5 text-[11px] font-semibold text-[#AAB0B6]">
          <WandSparkles className="h-3.5 w-3.5" />
          Bot Selection
        </div>
        <div className="flex gap-1.5">
          {(['scalper', 'meanReversion', 'breakout'] as const).map((preset) => (
            <button
              key={preset}
              className={`rounded border px-2 py-1 text-[10px] font-semibold ${
                selectedPreset === preset
                  ? 'border-[#00C076] bg-[#00C076]/15 text-[#00C076]'
                  : 'border-[#2B2F36] bg-[#141A22] text-[#AAB0B6] hover:bg-[#1B232E] hover:text-[#D9DEE3]'
              }`}
              onClick={() => onSelectPreset(preset)}
            >
              {PRESET_LABELS[preset]}
            </button>
          ))}
        </div>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {status === 'running' ? (
            <button onClick={onStop} className="inline-flex items-center gap-1 rounded bg-[#FF3B30] px-2 py-1 text-[10px] font-semibold text-white hover:brightness-110">
              <Square className="h-3 w-3" />
              Stop
            </button>
          ) : (
            <button onClick={onStart} className="inline-flex items-center gap-1 rounded bg-[#00C076] px-2 py-1 text-[10px] font-semibold text-black hover:brightness-110">
              <Play className="h-3 w-3" />
              Start
            </button>
          )}
          <button onClick={onOpenEditor} className="inline-flex items-center gap-1 rounded border border-[#2B2F36] bg-[#141A22] px-2 py-1 text-[10px] font-semibold text-[#D9DEE3] hover:bg-[#1B232E]">
            <SlidersHorizontal className="h-3 w-3" />
            Editor
          </button>
          <button onClick={onClear} className="rounded border border-[#FF3B30]/40 bg-[#FF3B30]/10 px-2 py-1 text-[10px] font-semibold text-[#FF3B30] hover:bg-[#FF3B30]/20">
            Clear
          </button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        <div className="mb-2 flex items-center gap-1.5 text-[11px] font-semibold text-[#AAB0B6]">
          <Trophy className="h-3.5 w-3.5 text-[#F0B90B]" />
          Bot Leaderboard
        </div>
        <div className="space-y-1.5">
          {leaderboard.map((row, idx) => (
            <div key={row.name} className="rounded border border-[#2B2F36] bg-[#111821] px-2 py-1.5">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <span className="text-[10px] text-[#7E8794]">#{idx + 1}</span>
                  <span className="text-[11px] font-semibold text-[#D9DEE3]">{row.name}</span>
                </div>
                <span className={`text-[11px] font-semibold ${row.pnl >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>{row.pnl.toFixed(2)}</span>
              </div>
              <div className="mt-1 flex items-center gap-2 text-[10px] text-[#7E8794]">
                <span>WR {row.winRate.toFixed(1)}%</span>
                <span>Trades {row.trades}</span>
                <span className={row.status === 'running' ? 'text-[#00C076]' : 'text-[#AAB0B6]'}>{row.status}</span>
              </div>
            </div>
          ))}
        </div>

        <div className="mt-3 border-t border-[#2B2F36] pt-3">
          <div className="mb-2 flex items-center gap-1.5 text-[11px] font-semibold text-[#AAB0B6]">
            <Activity className="h-3.5 w-3.5" />
            Bot Activity
          </div>
          <div className="space-y-1">
            {logs.slice(0, 6).map((log) => (
              <div key={log.id} className="rounded bg-[#10151D] px-2 py-1 text-[10px] text-[#AAB0B6]">
                {log.message}
              </div>
            ))}
            {logs.length === 0 && <div className="rounded bg-[#10151D] px-2 py-1 text-[10px] text-[#7E8794]">No activity yet</div>}
          </div>
        </div>
      </div>
    </div>
  )
}
