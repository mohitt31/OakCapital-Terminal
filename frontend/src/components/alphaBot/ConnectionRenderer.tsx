/* ── ConnectionRenderer – SVG edges between nodes ──────────── */
import type { BotEdge, BotNode } from '../../types/alphaBot'
import { getRegistryEntry } from '../../types/alphaBot'
import { NODE_W, HEADER_H, PORT_GAP, PORT_R } from './NodeRenderer'

type Props = {
  nodes: BotNode[]
  edges: BotEdge[]
  /** Temporary dragging edge while user draws a connection */
  draggingEdge: { fromX: number; fromY: number; toX: number; toY: number } | null
}

const portPos = (node: BotNode, portId: string, side: 'input' | 'output'): { x: number; y: number } | null => {
  const entry = getRegistryEntry(node.type)
  const ports = side === 'input' ? entry.inputs : entry.outputs
  const idx = ports.findIndex((p) => p.id === portId)
  if (idx < 0) return null
  const cy = node.y + HEADER_H + 12 + idx * PORT_GAP
  const cx = side === 'input' ? node.x - PORT_R : node.x + NODE_W + PORT_R
  return { x: cx, y: cy }
}

const bezierPath = (x1: number, y1: number, x2: number, y2: number) => {
  const dx = Math.abs(x2 - x1) * 0.5
  return `M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}`
}

export function ConnectionRenderer({ nodes, edges, draggingEdge }: Props) {
  const nodeMap = new Map(nodes.map((n) => [n.id, n]))

  return (
    <svg className="pointer-events-none absolute inset-0 h-full w-full" style={{ zIndex: 5 }}>
      <defs>
        <linearGradient id="edge-grad-num" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#3B82F6" stopOpacity="0.9" />
          <stop offset="100%" stopColor="#3B82F6" stopOpacity="0.4" />
        </linearGradient>
        <linearGradient id="edge-grad-bool" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#F0B90B" stopOpacity="0.9" />
          <stop offset="100%" stopColor="#F0B90B" stopOpacity="0.4" />
        </linearGradient>
        <filter id="edge-glow">
          <feGaussianBlur stdDeviation="2" result="glow" />
          <feMerge>
            <feMergeNode in="glow" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {edges.map((edge) => {
        const fromNode = nodeMap.get(edge.fromNode)
        const toNode = nodeMap.get(edge.toNode)
        if (!fromNode || !toNode) return null
        const from = portPos(fromNode, edge.fromPort, 'output')
        const to = portPos(toNode, edge.toPort, 'input')
        if (!from || !to) return null

        // Detect data type from source port
        const fromEntry = getRegistryEntry(fromNode.type)
        const sourcePort = fromEntry.outputs.find((p) => p.id === edge.fromPort)
        const isBool = sourcePort?.dataType === 'boolean'

        return (
          <path
            key={edge.id}
            d={bezierPath(from.x, from.y, to.x, to.y)}
            fill="none"
            stroke={`url(#edge-grad-${isBool ? 'bool' : 'num'})`}
            strokeWidth={2.5}
            filter="url(#edge-glow)"
          />
        )
      })}

      {/* Temporary edge being drawn */}
      {draggingEdge && (
        <path
          d={bezierPath(
            draggingEdge.fromX,
            draggingEdge.fromY,
            draggingEdge.toX,
            draggingEdge.toY,
          )}
          fill="none"
          stroke="#ffffff"
          strokeWidth={2}
          strokeDasharray="6 4"
          opacity={0.6}
        />
      )}
    </svg>
  )
}
