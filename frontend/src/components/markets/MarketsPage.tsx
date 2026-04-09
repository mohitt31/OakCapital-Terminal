import React, { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Search } from 'lucide-react';
import { MarketCard } from './MarketCard';
import { MarketListItem } from './MarketListItem';
import {
  indianStocks,
  indices,
  worldIndices,
  highestVolume,
  mostVolatile,
  gainers,
  losers
} from '../../data/marketData';

export const MarketsPage: React.FC = () => {
  const navigate = useNavigate();
  const [activeTimeframe, setActiveTimeframe] = useState('1D');
  const [activeTab, setActiveTab] = useState<'all' | 'indian' | 'global' | 'movers'>('all');
  const [query, setQuery] = useState('');
  const timeframes = ['1D', '1M', '3M', '1Y', '5Y', 'All'];
  const openTerminal = (symbol: string) => navigate(`/terminal?symbol=${encodeURIComponent(symbol)}`);

  const topMovers = useMemo(
    () => [...gainers, ...losers].sort((a, b) => Math.abs(b.changePercent) - Math.abs(a.changePercent)).slice(0, 14),
    [],
  );
  const filterRows = (rows: typeof highestVolume) => {
    const q = query.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter((row) => row.symbol.slice(0, 2).toLowerCase().includes(q));
  };
  const scannerRows = useMemo(() => {
    if (activeTab === 'indian') return filterRows([...indianStocks, ...highestVolume]);
    if (activeTab === 'global') return filterRows(worldIndices);
    if (activeTab === 'movers') return filterRows(topMovers);
    return filterRows([...indices, ...indianStocks, ...highestVolume, ...mostVolatile, ...topMovers]);
  }, [activeTab, query, topMovers]);
  const featuredCards = useMemo(
    () => [...indices.slice(0, 2), ...indianStocks.slice(0, 2), ...highestVolume.slice(0, 2)],
    [],
  );

  return (
    <div className="h-full w-full overflow-y-auto bg-[#0B0E11] text-[#D9DEE3]">
      <div className="mx-auto max-w-[1280px] px-5 py-5">
        <div className="mb-4 flex flex-wrap items-center gap-3">
          <h1 className="text-[24px] font-bold text-[#D9DEE3]">Markets</h1>
          <span className="rounded border border-[#2B2F36] bg-[#10141A] px-2 py-1 text-[11px] text-[#AAB0B6]">click any stock to open terminal</span>
        </div>

        <section className="mb-4 rounded border border-[#2B2F36] bg-[#10141A]">
          <header className="border-b border-[#2B2F36] px-3 py-2 text-[12px] font-semibold text-[#D9DEE3]">
            Featured
          </header>
          <div className="no-scrollbar flex snap-x snap-mandatory gap-3 overflow-x-auto p-3">
            {featuredCards.map((row, idx) => (
              <MarketCard key={`featured-${idx}-${row.symbol}`} data={row} onSelect={openTerminal} />
            ))}
          </div>
        </section>

        <div className="mb-4 flex flex-wrap items-center gap-2">
          {([
            { id: 'all', label: 'All' },
            { id: 'indian', label: 'Indian' },
            { id: 'global', label: 'Global' },
            { id: 'movers', label: 'Movers' },
          ] as const).map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`rounded border px-2.5 py-1 text-[11px] font-semibold ${
                activeTab === tab.id
                  ? 'border-[#00C076] bg-[#00C076]/15 text-[#00C076]'
                  : 'border-[#2B2F36] bg-[#10141A] text-[#AAB0B6] hover:bg-[#141A22]'
              }`}
            >
              {tab.label}
            </button>
          ))}
          <div className="ml-auto flex min-w-[230px] items-center gap-2 rounded border border-[#2B2F36] bg-[#10141A] px-2 py-1.5">
            <Search className="h-3.5 w-3.5 text-[#AAB0B6]" />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search symbol"
              className="w-full bg-transparent text-[12px] text-[#D9DEE3] outline-none placeholder:text-[#6D7480]"
            />
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-[1fr_1fr]">
          <section className="rounded border border-[#2B2F36] bg-[#10141A]">
            <header className="border-b border-[#2B2F36] px-3 py-2 text-[12px] font-semibold text-[#D9DEE3]">
              Market Scanner
            </header>
            <div className="no-scrollbar max-h-[520px] overflow-y-auto p-2">
              {scannerRows.map((row, idx) => (
                <MarketListItem key={`scanner-${idx}-${row.symbol}`} data={row} onSelect={openTerminal} />
              ))}
            </div>
          </section>

          <section className="rounded border border-[#2B2F36] bg-[#10141A]">
            <header className="flex items-center justify-between border-b border-[#2B2F36] px-3 py-2">
              <span className="text-[12px] font-semibold text-[#D9DEE3]">World Indices</span>
              <div className="flex items-center gap-1 rounded bg-[#0B0E11] p-1">
                {timeframes.map((tf) => (
                  <button
                    key={tf}
                    onClick={() => setActiveTimeframe(tf)}
                    className={`rounded px-2 py-[2px] text-[10px] font-semibold ${
                      activeTimeframe === tf ? 'bg-[#00C076]/15 text-[#00C076]' : 'text-[#AAB0B6] hover:bg-[#1C2128]'
                    }`}
                  >
                    {tf}
                  </button>
                ))}
              </div>
            </header>
            <div className="no-scrollbar max-h-[520px] overflow-y-auto p-2">
              {filterRows(worldIndices).map((row, idx) => (
                <MarketListItem key={`world-${idx}-${row.symbol}`} data={row} onSelect={openTerminal} />
              ))}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
};
