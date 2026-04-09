/* ── NodeConfigPanel – Hyperparameter editing sidebar ───────── */
import type { BotNode } from '../../types/alphaBot'
import { getRegistryEntry } from '../../types/alphaBot'
import { monoClass, scrollClass } from '../terminal/constants'

type Props = {
  node: BotNode | null
  onUpdateParams: (id: string, params: Record<string, number | string>) => void
  onDelete: (id: string) => void
  onClose: () => void
}

export function NodeConfigPanel({ node, onUpdateParams, onDelete, onClose }: Props) {
  if (!node) {
    return (
      <aside className="flex h-full w-[220px] shrink-0 flex-col items-center justify-center border-l border-[#2B2F36] bg-[#10141A] text-[11px] text-[#6D7480]">
        <div className="text-center px-4">
          <div className="text-[32px] mb-2 opacity-30">⚙️</div>
          Select a node to edit its parameters
        </div>
      </aside>
    )
  }

  const entry = getRegistryEntry(node.type)

  return (
    <aside className={`flex h-full w-[220px] shrink-0 flex-col border-l border-[#2B2F36] bg-[#10141A] overflow-y-auto ${scrollClass}`}>
      {/* Header */}
      <header className="shrink-0 border-b border-[#2B2F36] px-3 py-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span
              className="h-3 w-3 rounded-full"
              style={{ background: entry.color }}
            />
            <span className="text-[12px] font-bold text-[#D9DEE3]">{node.label}</span>
          </div>
          <button
            className="text-[#6D7480] hover:text-white transition-colors text-[14px]"
            onClick={onClose}
            title="Close config"
          >
            ✕
          </button>
        </div>
        <div className="mt-1 text-[9px] text-[#6D7480] uppercase tracking-wide">
          {entry.category} · {entry.type}
        </div>
      </header>

      {/* Port info */}
      <div className="shrink-0 border-b border-[#2B2F36] px-3 py-2">
        <div className="text-[10px] font-semibold tracking-wide text-[#AAB0B6] mb-1">PORTS</div>
        <div className="space-y-0.5 text-[10px]">
          {entry.inputs.length > 0 && (
            <div className="flex gap-1">
              <span className="text-[#6D7480]">In:</span>
              <span className="text-[#D9DEE3]">{entry.inputs.map((p) => p.label).join(', ')}</span>
            </div>
          )}
          {entry.outputs.length > 0 && (
            <div className="flex gap-1">
              <span className="text-[#6D7480]">Out:</span>
              <span className="text-[#D9DEE3]">{entry.outputs.map((p) => p.label).join(', ')}</span>
            </div>
          )}
        </div>
      </div>

      {/* Parameters */}
      {entry.params.length > 0 ? (
        <div className="flex-1 p-3 space-y-3">
          <div className="text-[10px] font-semibold tracking-wide text-[#AAB0B6]">PARAMETERS</div>
          {entry.params.map((param) => {
            const val = node.params[param.key] ?? param.default
            return (
              <div key={param.key}>
                <label className="block text-[10px] text-[#AAB0B6] mb-1">{param.label}</label>
                {param.type === 'select' ? (
                  <select
                    className={`w-full rounded border border-[#2B2F36] bg-[#0B0E11] px-2 py-1.5 text-[11px] text-[#D9DEE3] ${monoClass} focus:border-[#3B82F6] focus:outline-none`}
                    value={String(val)}
                    onChange={(e) =>
                      onUpdateParams(node.id, { [param.key]: e.target.value })
                    }
                  >
                    {param.options?.map((opt) => (
                      <option key={opt.value} value={opt.value}>
                        {opt.label}
                      </option>
                    ))}
                  </select>
                ) : (
                  <div className="space-y-1">
                    <input
                      type="number"
                      className={`w-full rounded border border-[#2B2F36] bg-[#0B0E11] px-2 py-1.5 text-[11px] text-[#D9DEE3] ${monoClass} focus:border-[#3B82F6] focus:outline-none`}
                      value={val}
                      min={param.min}
                      max={param.max}
                      step={param.step}
                      onChange={(e) =>
                        onUpdateParams(node.id, { [param.key]: Number(e.target.value) })
                      }
                    />
                    {param.min !== undefined && param.max !== undefined && (
                      <input
                        type="range"
                        className="w-full accent-[#3B82F6] h-1"
                        value={Number(val)}
                        min={param.min}
                        max={param.max}
                        step={param.step}
                        onChange={(e) =>
                          onUpdateParams(node.id, { [param.key]: Number(e.target.value) })
                        }
                      />
                    )}
                    {param.min !== undefined && param.max !== undefined && (
                      <div className={`flex justify-between text-[9px] text-[#6D7480] ${monoClass}`}>
                        <span>{param.min}</span>
                        <span>{param.max}</span>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      ) : (
        <div className="flex-1 flex items-center justify-center p-3 text-[10px] text-[#6D7480]">
          No configurable parameters
        </div>
      )}

      {/* Node ID & Delete */}
      <div className="shrink-0 border-t border-[#2B2F36] p-3 space-y-2">
        <div className={`text-[9px] text-[#6D7480] truncate ${monoClass}`}>
          ID: {node.id}
        </div>
        <button
          className="w-full rounded border border-[#FF3B30]/40 bg-[#FF3B30]/10 px-2 py-1.5 text-[11px] font-semibold text-[#FF3B30] transition-colors hover:bg-[#FF3B30]/20"
          onClick={() => onDelete(node.id)}
        >
          Delete Node
        </button>
      </div>
    </aside>
  )
}
