import React, { useMemo } from 'react';
import type { StockData } from '../../data/marketData';

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
    const y = h - ((p - min) / range) * (h * 0.7) - h * 0.15;
    return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(' ');
}

function symbolColor(s: string): string {
  const c = ['#6366f1','#ec4899','#f59e0b','#10b981','#3b82f6','#8b5cf6','#ef4444','#14b8a6','#f97316','#06b6d4','#84cc16','#e879f9'];
  let h = 0;
  for (let i = 0; i < s.length; i++) h = s.charCodeAt(i) + ((h << 5) - h);
  return c[Math.abs(h) % c.length];
}

export const MarketListItem: React.FC<{ data: StockData; showBadge?: boolean; onSelect?: (symbol: string) => void }> = ({ data, showBadge = false, onSelect }) => {
  const isPositive = data.changePercent >= 0;
  const color = useMemo(() => symbolColor(data.symbol), [data.symbol]);
  const pts = useMemo(() => generatePoints(data.symbol, 20, isPositive), [data.symbol, isPositive]);
  const lineColor = isPositive ? '#22c55e' : '#ef4444';
  const sparkW = 64, sparkH = 28;
  const sparkPath = toPath(pts, sparkW, sparkH);
  
  return (
    <div className="group flex cursor-pointer items-center gap-3 rounded-xl px-3 py-3 transition-all duration-200 hover:bg-[#1C2128]" onClick={() => onSelect?.(data.symbol)}>
      {/* Logo */}
      <div
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-[10px] font-black text-white transition-transform duration-200 group-hover:scale-110"
        style={{ background: `linear-gradient(135deg, ${color}, ${color}88)` }}
      >
        {data.symbol.slice(0, 2).toUpperCase()}
      </div>

      {/* Name */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="truncate text-[13px] font-bold text-[#D9DEE3]">{data.name || data.symbol}</span>
          {data.isDerived && (
            <span className="flex h-[13px] min-w-[13px] items-center justify-center rounded bg-amber-500/90 text-[7px] font-black text-black px-0.5">D</span>
          )}
        </div>
        <span className="block text-[11px] text-[#AAB0B6] font-medium">{data.name ? data.symbol : ''}</span>
      </div>

      {/* Sparkline */}
      <svg className="h-[28px] w-[64px] shrink-0" viewBox={`0 0 ${sparkW} ${sparkH}`} preserveAspectRatio="none">
        <path d={sparkPath} fill="none" stroke={lineColor} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" opacity="0.8" />
      </svg>

      {/* Price */}
      <div className="flex w-[85px] shrink-0 flex-col items-end">
        <span className="text-[13px] font-bold text-[#D9DEE3]">
          {data.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
        </span>
        <span className="text-[10px] text-[#AAB0B6] uppercase">{data.currency}</span>
      </div>

      {/* Change */}
      {showBadge ? (
        <div className={`flex w-[72px] shrink-0 items-center justify-center rounded-lg py-1.5 text-[12px] font-bold transition-transform duration-200 group-hover:scale-105 ${isPositive ? 'bg-[#00C076]/90 text-black' : 'bg-[#FF3B30]/90 text-white'}`}>
          {isPositive ? '+' : ''}{data.changePercent.toFixed(2)}%
        </div>
      ) : (
        <span className={`w-[60px] shrink-0 text-right text-[13px] font-bold ${isPositive ? 'text-[#00C076]' : 'text-[#FF3B30]'}`}>
          {isPositive ? '+' : ''}{data.changePercent.toFixed(2)}%
        </span>
      )}
    </div>
  );
};
