/* ── NodePalette – Draggable node list sidebar ─────────────── */
import { NODE_REGISTRY, type NodeCategory, type NodeType } from '../../types/alphaBot'
import { scrollClass } from '../terminal/constants'

type Props = {
  onAddNode: (type: NodeType, x: number, y: number) => void
}

const CATEGORIES: { key: NodeCategory; label: string; icon: string }[] = [
  { key: 'data', label: 'Data Sources', icon: '📡' },
  { key: 'indicator', label: 'Indicators', icon: '📈' },
  { key: 'condition', label: 'Conditions', icon: '⚡' },
  { key: 'action', label: 'Actions', icon: '🎯' },
]

export function NodePalette({ onAddNode }: Props) {
  const handleDragStart = (e: React.DragEvent, type: NodeType) => {
    e.dataTransfer.setData('application/alphabot-node', type)
    e.dataTransfer.effectAllowed = 'copy'
  }

  const handleDoubleClick = (type: NodeType) => {
    // Quick-add at a default position
    onAddNode(type, 200 + Math.random() * 200, 100 + Math.random() * 200)
  }

  return (
    <aside className={`flex h-full w-[200px] shrink-0 flex-col border-r border-[#2B2F36] bg-[#10141A] overflow-y-auto ${scrollClass}`}>
      <header className="shrink-0 border-b border-[#2B2F36] px-3 py-2 text-[11px] font-bold tracking-widest text-[#AAB0B6]">
        NODE PALETTE
      </header>

      <div className="flex-1 p-2 space-y-3">
        {CATEGORIES.map((cat) => {
          const items = NODE_REGISTRY.filter((n) => n.category === cat.key)
          if (items.length === 0) return null
          return (
            <div key={cat.key}>
              <div className="mb-1.5 flex items-center gap-1.5 text-[10px] font-semibold tracking-wide text-[#6D7480] uppercase">
                <span>{cat.icon}</span>
                <span>{cat.label}</span>
              </div>
              <div className="space-y-1">
                {items.map((entry) => (
                  <div
                    key={entry.type}
                    draggable
                    onDragStart={(e) => handleDragStart(e, entry.type)}
                    onDoubleClick={() => handleDoubleClick(entry.type)}
                    className="group flex cursor-grab items-center gap-2 rounded-md border border-[#2B2F36] bg-[#1A1E26] px-2.5 py-1.5 text-[11px] text-[#D9DEE3] transition-all hover:border-[#3A404A] hover:bg-[#22272E] active:cursor-grabbing"
                  >
                    <span
                      className="h-2.5 w-2.5 shrink-0 rounded-full transition-transform group-hover:scale-125"
                      style={{ background: entry.color }}
                    />
                    <span className="truncate">{entry.label}</span>
                    <span className="ml-auto text-[9px] text-[#6D7480]">
                      {entry.inputs.length > 0 ? `${entry.inputs.length}→` : ''}
                      {entry.outputs.length > 0 ? `→${entry.outputs.length}` : ''}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )
        })}
      </div>

      <footer className="shrink-0 border-t border-[#2B2F36] px-3 py-2 text-[9px] text-[#6D7480]">
        Drag nodes onto canvas or double-click to add
      </footer>
    </aside>
  )
}
