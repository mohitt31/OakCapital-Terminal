/* ── AlphaBotEditor – Full-screen node-based strategy builder ── */
import { useCallback, useRef, useState } from 'react'
import type { BotNode, NodeType } from '../../types/alphaBot'
import { createNode, uid, type BotStatus, type BotLog } from '../../hooks/useAlphaBot'
import { NodeRenderer } from './NodeRenderer'
import { ConnectionRenderer } from './ConnectionRenderer'
import { NodePalette } from './NodePalette'
import { NodeConfigPanel } from './NodeConfigPanel'
import { monoClass, scrollClass } from '../terminal/constants'

type DraggingEdge = { fromX: number; fromY: number; toX: number; toY: number }
type PendingPort = { nodeId: string; portId: string; side: 'input' | 'output'; x: number; y: number }

type Props = {
  nodes: BotNode[]
  edges: { id: string; fromNode: string; fromPort: string; toNode: string; toPort: string }[]
  status: BotStatus
  logs: BotLog[]
  pnl: number
  onAddNode: (node: BotNode) => void
  onRemoveNode: (id: string) => void
  onUpdateParams: (id: string, params: Record<string, number | string>) => void
  onMoveNode: (id: string, x: number, y: number) => void
  onAddEdge: (edge: { id: string; fromNode: string; fromPort: string; toNode: string; toPort: string }) => void
  onClear: () => void
  onStart: () => void
  onStop: () => void
  onClose: () => void
}

export function AlphaBotEditor({
  nodes,
  edges,
  status,
  logs,
  pnl,
  onAddNode,
  onRemoveNode,
  onUpdateParams,
  onMoveNode,
  onAddEdge,
  onClear,
  onStart,
  onStop,
  onClose,
}: Props) {
  const canvasRef = useRef<HTMLDivElement>(null)
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [draggingEdge, setDraggingEdge] = useState<DraggingEdge | null>(null)
  const [showLogs, setShowLogs] = useState(false)

  // ── Node dragging state ──
  const dragState = useRef<{
    nodeId: string
    offsetX: number
    offsetY: number
  } | null>(null)

  // ── Port connection state ──
  const pendingPortRef = useRef<PendingPort | null>(null)

  // ── Canvas offset for coordinate transforms ──
  const getCanvasOffset = useCallback(() => {
    if (!canvasRef.current) return { x: 0, y: 0 }
    const rect = canvasRef.current.getBoundingClientRect()
    return { x: rect.left, y: rect.top }
  }, [])

  // ── Add node from palette ──
  const handleAddNode = useCallback(
    (type: NodeType, x: number, y: number) => {
      const node = createNode(type, x, y)
      onAddNode(node)
    },
    [onAddNode],
  )

  // ── Drop from palette ──
  const handleCanvasDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      const type = e.dataTransfer.getData('application/alphabot-node') as NodeType
      if (!type) return
      const offset = getCanvasOffset()
      const x = e.clientX - offset.x - 82 // center the node
      const y = e.clientY - offset.y - 14
      handleAddNode(type, Math.max(0, x), Math.max(0, y))
    },
    [getCanvasOffset, handleAddNode],
  )

  const handleCanvasDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'copy'
  }, [])

  // ── Node drag ──
  const handleNodeDragStart = useCallback((nodeId: string, offsetX: number, offsetY: number) => {
    dragState.current = { nodeId, offsetX, offsetY }
  }, [])

  const handleCanvasPointerMove = useCallback(
    (e: React.PointerEvent) => {
      // Node dragging
      if (dragState.current) {
        const offset = getCanvasOffset()
        const x = e.clientX - offset.x - dragState.current.offsetX
        const y = e.clientY - offset.y - dragState.current.offsetY
        onMoveNode(dragState.current.nodeId, Math.max(0, x), Math.max(0, y))
      }
      // Edge dragging
      if (pendingPortRef.current) {
        const offset = getCanvasOffset()
        setDraggingEdge({
          fromX: pendingPortRef.current.x - offset.x,
          fromY: pendingPortRef.current.y - offset.y,
          toX: e.clientX - offset.x,
          toY: e.clientY - offset.y,
        })
      }
    },
    [getCanvasOffset, onMoveNode],
  )

  const handleCanvasPointerUp = useCallback(() => {
    dragState.current = null
    pendingPortRef.current = null
    setDraggingEdge(null)
  }, [])

  // ── Port connection ──
  const handlePortPointerDown = useCallback(
    (nodeId: string, portId: string, side: 'output' | 'input', x: number, y: number) => {
      pendingPortRef.current = { nodeId, portId, side, x, y }
    },
    [],
  )

  const handlePortPointerUp = useCallback(
    (nodeId: string, portId: string, side: 'output' | 'input') => {
      const pending = pendingPortRef.current
      if (!pending) return
      // Must connect output→input, different nodes
      if (pending.side === side) return
      if (pending.nodeId === nodeId) return

      const fromNode = side === 'input' ? pending.nodeId : nodeId
      const fromPort = side === 'input' ? pending.portId : portId
      const toNode = side === 'input' ? nodeId : pending.nodeId
      const toPort = side === 'input' ? portId : pending.portId

      onAddEdge({
        id: uid(),
        fromNode,
        fromPort,
        toNode,
        toPort,
      })

      pendingPortRef.current = null
      setDraggingEdge(null)
    },
    [onAddEdge],
  )

  const handleCanvasClick = useCallback(() => {
    setSelectedNode(null)
  }, [])

  const selectedNodeObj = nodes.find((n) => n.id === selectedNode) ?? null

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-[#0B0E11]">
      {/* ── Top Toolbar ── */}
      <header className="flex shrink-0 items-center justify-between border-b border-[#2B2F36] bg-[#10141A] px-4 py-2">
        <div className="flex items-center gap-3">
          <span className="text-[14px] font-bold text-[#D9DEE3]">🤖 Alpha Bot Strategy Editor</span>
          <span className="rounded border border-[#2B2F36] bg-[#0B0E11] px-2 py-0.5 text-[10px] text-[#6D7480]">
            {nodes.length} nodes · {edges.length} edges
          </span>
        </div>

        <div className="flex items-center gap-2">
          {/* P&L */}
          <div className="rounded border border-[#2B2F36] bg-[#0B0E11] px-3 py-1 text-[11px]">
            <span className="text-[#AAB0B6]">Bot P&L: </span>
            <span className={`${monoClass} font-bold ${pnl >= 0 ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>
              ${pnl.toFixed(2)}
            </span>
          </div>

          {/* Status indicator */}
          <div className="flex items-center gap-1.5 rounded border border-[#2B2F36] bg-[#0B0E11] px-3 py-1 text-[11px]">
            <span
              className="h-2 w-2 rounded-full"
              style={{
                background: status === 'running' ? '#00C076' : status === 'error' ? '#FF3B30' : '#6D7480',
                boxShadow: status === 'running' ? '0 0 6px #00C076' : 'none',
                animation: status === 'running' ? 'pulse 1.5s infinite' : 'none',
              }}
            />
            <span className="text-[#AAB0B6] capitalize">{status}</span>
          </div>

          {/* Controls */}
          {status === 'running' ? (
            <button
              className="rounded bg-[#FF3B30] px-3 py-1 text-[11px] font-bold text-white transition-colors hover:bg-[#FF3B30]/80"
              onClick={onStop}
            >
              ⏹ Stop
            </button>
          ) : (
            <button
              className="rounded bg-[#00C076] px-3 py-1 text-[11px] font-bold text-black transition-colors hover:bg-[#00C076]/80"
              onClick={onStart}
            >
              ▶ Start
            </button>
          )}

          <button
            className="rounded border border-[#2B2F36] bg-[#1A1E26] px-3 py-1 text-[11px] text-[#AAB0B6] transition-colors hover:bg-[#22272E] hover:text-white"
            onClick={() => setShowLogs((p) => !p)}
          >
            {showLogs ? '📋 Hide Logs' : '📋 Logs'}
          </button>

          <button
            className="rounded border border-[#FF3B30]/30 bg-[#FF3B30]/10 px-3 py-1 text-[11px] text-[#FF3B30] transition-colors hover:bg-[#FF3B30]/20"
            onClick={onClear}
          >
            🗑 Clear
          </button>

          <button
            className="rounded border border-[#2B2F36] bg-[#1A1E26] px-3 py-1 text-[11px] text-[#AAB0B6] transition-colors hover:bg-[#22272E] hover:text-white"
            onClick={onClose}
          >
            ✕ Close
          </button>
        </div>
      </header>

      {/* ── Main content area ── */}
      <div className="flex min-h-0 flex-1">
        {/* Left: node palette */}
        <NodePalette onAddNode={handleAddNode} />

        {/* Center: canvas */}
        <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">
          <div
            ref={canvasRef}
            className="absolute inset-0"
            style={{
              backgroundImage:
                'radial-gradient(circle, #2B2F3622 1px, transparent 1px)',
              backgroundSize: '24px 24px',
            }}
            onPointerMove={handleCanvasPointerMove}
            onPointerUp={handleCanvasPointerUp}
            onDrop={handleCanvasDrop}
            onDragOver={handleCanvasDragOver}
            onClick={handleCanvasClick}
          >
            {/* Edges layer */}
            <ConnectionRenderer
              nodes={nodes}
              edges={edges}
              draggingEdge={draggingEdge}
            />

            {/* Nodes layer */}
            {nodes.map((node) => (
              <NodeRenderer
                key={node.id}
                node={node}
                selected={selectedNode === node.id}
                onSelect={setSelectedNode}
                onDragStart={handleNodeDragStart}
                onPortPointerDown={handlePortPointerDown}
                onPortPointerUp={handlePortPointerUp}
                onDelete={onRemoveNode}
              />
            ))}

            {/* Empty state */}
            {nodes.length === 0 && (
              <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                <div className="text-center space-y-2 opacity-40">
                  <div className="text-[48px]">🔗</div>
                  <div className="text-[14px] text-[#AAB0B6]">
                    Drag nodes from the palette to build your strategy
                  </div>
                  <div className="text-[11px] text-[#6D7480]">
                    Connect outputs → inputs to create the data flow
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Log panel overlay at bottom */}
          {showLogs && (
            <div className={`absolute bottom-0 left-0 right-0 h-[160px] border-t border-[#2B2F36] bg-[#10141A]/95 backdrop-blur-sm overflow-y-auto ${scrollClass}`}>
              <div className="sticky top-0 border-b border-[#2B2F36] bg-[#10141A] px-3 py-1 text-[10px] font-bold tracking-widest text-[#AAB0B6]">
                BOT ACTIVITY LOG
              </div>
              <div className="p-2 space-y-0.5">
                {logs.length === 0 ? (
                  <div className="text-[10px] text-[#6D7480]">No activity yet</div>
                ) : (
                  logs.map((log) => (
                    <div
                      key={log.id}
                      className={`flex gap-2 rounded px-2 py-0.5 text-[10px] ${monoClass} ${
                        log.type === 'trade'
                          ? 'bg-[#00C076]/5 text-[#00C076]'
                          : log.type === 'error'
                            ? 'bg-[#FF3B30]/5 text-[#FF3B30]'
                            : 'text-[#AAB0B6]'
                      }`}
                    >
                      <span className="shrink-0 text-[#6D7480]">
                        {new Date(log.time).toLocaleTimeString()}
                      </span>
                      <span>{log.message}</span>
                    </div>
                  ))
                )}
              </div>
            </div>
          )}
        </div>

        {/* Right: config panel */}
        <NodeConfigPanel
          node={selectedNodeObj}
          onUpdateParams={onUpdateParams}
          onDelete={(id) => {
            onRemoveNode(id)
            if (selectedNode === id) setSelectedNode(null)
          }}
          onClose={() => setSelectedNode(null)}
        />
      </div>

      {/* Pulse animation for running indicator */}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.4; }
        }
      `}</style>
    </div>
  )
}
