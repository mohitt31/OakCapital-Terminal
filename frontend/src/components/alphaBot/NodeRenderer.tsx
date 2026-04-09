/* ── NodeRenderer – Single node brick ──────────────────────── */
import { useCallback, useRef } from 'react'
import type { BotNode } from '../../types/alphaBot'
import { getRegistryEntry } from '../../types/alphaBot'
import { monoClass } from '../terminal/constants'

const PORT_R = 6
const PORT_GAP = 22
const NODE_W = 164
const HEADER_H = 28

type Props = {
  node: BotNode
  selected: boolean
  onSelect: (id: string) => void
  onDragStart: (id: string, offsetX: number, offsetY: number) => void
  onPortPointerDown: (nodeId: string, portId: string, side: 'output' | 'input', x: number, y: number) => void
  onPortPointerUp: (nodeId: string, portId: string, side: 'output' | 'input') => void
  onDelete: (id: string) => void
}

export function NodeRenderer({
  node,
  selected,
  onSelect,
  onDragStart,
  onPortPointerDown,
  onPortPointerUp,
  onDelete,
}: Props) {
  const entry = getRegistryEntry(node.type)
  const maxPorts = Math.max(entry.inputs.length, entry.outputs.length, 1)
  const bodyH = maxPorts * PORT_GAP + 10
  const totalH = HEADER_H + bodyH
  const dragRef = useRef(false)

  const handlePointerDown = useCallback(
    (e: React.PointerEvent) => {
      if ((e.target as HTMLElement).dataset.port) return
      e.stopPropagation()
      dragRef.current = true
      const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
      onDragStart(node.id, e.clientX - rect.left, e.clientY - rect.top)
    },
    [node.id, onDragStart],
  )

  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation()
      if (!dragRef.current) onSelect(node.id)
      dragRef.current = false
    },
    [node.id, onSelect],
  )

  const portCenter = (index: number) => HEADER_H + 12 + index * PORT_GAP

  return (
    <div
      className="pointer-events-auto absolute select-none"
      style={{
        left: node.x,
        top: node.y,
        width: NODE_W,
        height: totalH,
        zIndex: selected ? 20 : 10,
      }}
      onPointerDown={handlePointerDown}
      onClick={handleClick}
    >
      {/* Glow ring when selected */}
      <div
        className="absolute inset-0 rounded-lg transition-shadow"
        style={{
          boxShadow: selected ?
            `0 0 0 2px ${entry.color}88, 0 0 20px ${entry.color}33` :
            '0 0 0 1px #2B2F36',
        }}
      />

      {/* Card body */}
      <div className="relative h-full rounded-lg bg-[#1A1E26] overflow-hidden">
        {/* Header */}
        <div
          className="flex items-center justify-between px-2.5 text-[11px] font-bold tracking-wide text-white"
          style={{ height: HEADER_H, background: `${entry.color}CC` }}
        >
          <span className="truncate">{node.label}</span>
          <button
            className="ml-1 flex h-4 w-4 items-center justify-center rounded text-white/60 hover:bg-white/20 hover:text-white"
            onClick={(e) => { e.stopPropagation(); onDelete(node.id) }}
            title="Delete node"
          >
            ×
          </button>
        </div>

        {/* Port area */}
        <div className="relative" style={{ height: bodyH }}>
          {/* Input ports */}
          {entry.inputs.map((port, i) => {
            const cy = portCenter(i) - HEADER_H
            return (
              <div key={port.id}>
                <div
                  data-port="input"
                  className="absolute cursor-crosshair rounded-full border-2 border-[#2B2F36] transition-colors hover:border-white hover:scale-125"
                  style={{
                    left: -PORT_R,
                    top: cy - PORT_R,
                    width: PORT_R * 2,
                    height: PORT_R * 2,
                    background: port.dataType === 'boolean' ? '#F0B90B' : '#3B82F6',
                  }}
                  onPointerDown={(e) => {
                    e.stopPropagation()
                    const rect = e.currentTarget.getBoundingClientRect()
                    onPortPointerDown(node.id, port.id, 'input', rect.left + PORT_R, rect.top + PORT_R)
                  }}
                  onPointerUp={(e) => {
                    e.stopPropagation()
                    onPortPointerUp(node.id, port.id, 'input')
                  }}
                />
                <span
                  className={`absolute text-[9px] text-[#AAB0B6] ${monoClass}`}
                  style={{ left: PORT_R + 4, top: cy - 6 }}
                >
                  {port.label}
                </span>
              </div>
            )
          })}

          {/* Output ports */}
          {entry.outputs.map((port, i) => {
            const cy = portCenter(i) - HEADER_H
            return (
              <div key={port.id}>
                <div
                  data-port="output"
                  className="absolute cursor-crosshair rounded-full border-2 border-[#2B2F36] transition-colors hover:border-white hover:scale-125"
                  style={{
                    right: -PORT_R,
                    top: cy - PORT_R,
                    width: PORT_R * 2,
                    height: PORT_R * 2,
                    background: port.dataType === 'boolean' ? '#F0B90B' : '#3B82F6',
                  }}
                  onPointerDown={(e) => {
                    e.stopPropagation()
                    const rect = e.currentTarget.getBoundingClientRect()
                    onPortPointerDown(node.id, port.id, 'output', rect.left + PORT_R, rect.top + PORT_R)
                  }}
                  onPointerUp={(e) => {
                    e.stopPropagation()
                    onPortPointerUp(node.id, port.id, 'output')
                  }}
                />
                <span
                  className={`absolute text-right text-[9px] text-[#AAB0B6] ${monoClass}`}
                  style={{ right: PORT_R + 4, top: cy - 6 }}
                >
                  {port.label}
                </span>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

// Exported constants for edge calculations
export { NODE_W, HEADER_H, PORT_GAP, PORT_R }
