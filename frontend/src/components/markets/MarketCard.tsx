import React, { useMemo, useState } from 'react';
import type { StockData } from '../../data/marketData';

/* ── Deterministic sparkline from symbol ── */
function generatePoints(symbol: string, count: number, isPositive: boolean): number[] {
  let seed = 0;
  for (let i = 0; i < symbol.length; i++) seed += symbol.charCodeAt(i) * (i + 1);
  const pts: number[] = [];
  let val = 50;
  for (let i = 0; i < count; i++) {
    seed = (seed * 16807 + 12345) % 2147483647;
    val += ((seed % 100) - 50) * 0.3;
    val = Math.max(10, Math.min(90, val));
    pts.push(val);
  }
  // ensure direction
  if (isPositive && pts[pts.length - 1] < pts[0]) pts.reverse();
  if (!isPositive && pts[pts.length - 1] > pts[0]) pts.reverse();
  return pts;
}

function toPath(pts: number[], w: number, h: number): string {
  const stepX = w / (pts.length - 1);
  const min = Math.min(...pts);
  const max = Math.max(...pts);
  const range = max - min || 1;
  return pts.map((p, i) => {
    const x = i * stepX;
    const y = h - ((p - min) / range) * (h * 0.8) - h * 0.1;
    return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(' ');
}

function toAreaPath(pts: number[], w: number, h: number): string {
  return toPath(pts, w, h) + ` L${w},${h} L0,${h} Z`;
}

function symbolColor(s: string): string {
  const c = ['#6366f1','#ec4899','#f59e0b','#10b981','#3b82f6','#8b5cf6','#ef4444','#14b8a6','#f97316','#06b6d4','#84cc16','#e879f9'];
  let h = 0;
  for (let i = 0; i < s.length; i++) h = s.charCodeAt(i) + ((h << 5) - h);
  return c[Math.abs(h) % c.length];
}

export const MarketCard: React.FC<{ data: StockData; onSelect?: (symbol: string) => void }> = ({ data, onSelect }) => {
  const [hovered, setHovered] = useState(false);
  const isPositive = data.changePercent >= 0;
  const color = useMemo(() => symbolColor(data.symbol), [data.symbol]);
  const pts = useMemo(() => generatePoints(data.symbol, 30, isPositive), [data.symbol, isPositive]);
  const lineColor = isPositive ? '#22c55e' : '#ef4444';

  const chartW = 240, chartH = 60;
  const linePath = toPath(pts, chartW, chartH);
  const areaPath = toAreaPath(pts, chartW, chartH);

  return (
    <div
      className="group relative flex w-[260px] shrink-0 flex-col overflow-hidden rounded-2xl border border-[#2B2F36] bg-[#10141A] transition-all duration-300 hover:border-[#3A404A] hover:bg-[#141A22] hover:shadow-xl hover:shadow-black/30 hover:-translate-y-0.5"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onClick={() => onSelect?.(data.symbol)}
      style={{ cursor: 'pointer' }}
    >
      {/* Top Info */}
      <div className="flex items-start gap-3 px-4 pt-4 pb-1 z-10">
        <div
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-[11px] font-black text-white shadow-lg transition-transform duration-300 group-hover:scale-110"
          style={{ background: `linear-gradient(135deg, ${color}, ${color}99)` }}
        >
          {data.symbol.slice(0, 2).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
          <span className="truncate text-[14px] font-bold text-[#D9DEE3]">{data.symbol}</span>
            {data.isDerived && (
              <span className="flex h-[15px] min-w-[15px] shrink-0 items-center justify-center rounded bg-amber-500/90 text-[7px] font-black text-black px-0.5">D</span>
            )}
          </div>
          {data.name && (
            <span className="block truncate text-[11px] text-[#AAB0B6] leading-tight mt-0.5">{data.name}</span>
          )}
        </div>
      </div>

      {/* Chart Area */}
      <div className="relative h-[60px] mt-1">
        <svg className="w-full h-full" viewBox={`0 0 ${chartW} ${chartH}`} preserveAspectRatio="none">
          <defs>
            <linearGradient id={`grad-${data.symbol.replace(/[^a-zA-Z]/g,'')}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={lineColor} stopOpacity={hovered ? 0.25 : 0.12} />
              <stop offset="100%" stopColor={lineColor} stopOpacity="0" />
            </linearGradient>
          </defs>
          <path d={areaPath} fill={`url(#grad-${data.symbol.replace(/[^a-zA-Z]/g,'')})`} />
          <path
            d={linePath}
            fill="none"
            stroke={lineColor}
            strokeWidth={hovered ? 2.5 : 1.8}
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{ transition: 'stroke-width 0.3s' }}
          />
          {/* End dot */}
          {(() => {
            const lastPt = pts[pts.length - 1];
            const min = Math.min(...pts);
            const max = Math.max(...pts);
            const range = max - min || 1;
            const cy = chartH - ((lastPt - min) / range) * (chartH * 0.8) - chartH * 0.1;
            return (
              <circle
                cx={chartW}
                cy={cy}
                r={hovered ? 4 : 2.5}
                fill={lineColor}
                stroke="#10141A"
                strokeWidth="2"
                className="transition-all duration-300"
              />
            );
          })()}
        </svg>
      </div>

      {/* Bottom Price */}
      <div className="flex items-baseline justify-between px-4 pb-3 pt-1 z-10">
        <div className="flex items-baseline gap-1.5">
            <span className="text-[16px] font-bold text-[#D9DEE3]">
            {data.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
          </span>
          <span className="text-[10px] font-medium text-[#AAB0B6] uppercase">{data.currency}</span>
        </div>
        <span className={`text-[13px] font-bold ${isPositive ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>
          {isPositive ? '+' : ''}{data.changePercent.toFixed(2)}%
        </span>
      </div>
    </div>
  );
};
