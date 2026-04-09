import React, { useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { ChevronRight, ChevronLeft } from 'lucide-react';

interface MarketSectionProps {
  title: string;
  children: ReactNode;
  icon?: ReactNode;
  seeAllLink?: string;
  seeAllText?: string;
  horizontalScroll?: boolean;
  filters?: ReactNode;
}

export const MarketSection: React.FC<MarketSectionProps> = ({
  title,
  children,
  icon,
  seeAllLink,
  seeAllText = "See all",
  horizontalScroll = true,
  filters
}) => {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(true);

  const checkScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    setCanScrollLeft(el.scrollLeft > 4);
    setCanScrollRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 4);
  };

  const scroll = (dir: 'left' | 'right') => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollBy({ left: dir === 'left' ? -300 : 300, behavior: 'smooth' });
  };

  return (
    <div className="mb-8 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          {icon}
          <h2 className="group flex cursor-pointer items-center text-[18px] font-bold text-[#D9DEE3] hover:text-white transition-colors">
            {title}
            <ChevronRight className="ml-1 h-4 w-4 opacity-0 transition-all duration-200 group-hover:opacity-60" />
          </h2>
          {filters}
        </div>
        {horizontalScroll && (
          <div className="flex items-center gap-1.5">
            <button
              onClick={() => scroll('left')}
              className={`flex h-8 w-8 items-center justify-center rounded-xl border transition-all duration-200 ${canScrollLeft ? 'border-[#2B2F36] text-[#AAB0B6] hover:bg-[#1C2128] hover:text-[#D9DEE3]' : 'border-[#2B2F36]/40 text-[#AAB0B6]/30 cursor-default'}`}
              disabled={!canScrollLeft}
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <button
              onClick={() => scroll('right')}
              className={`flex h-8 w-8 items-center justify-center rounded-xl border transition-all duration-200 ${canScrollRight ? 'border-[#2B2F36] text-[#AAB0B6] hover:bg-[#1C2128] hover:text-[#D9DEE3]' : 'border-[#2B2F36]/40 text-[#AAB0B6]/30 cursor-default'}`}
              disabled={!canScrollRight}
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        )}
      </div>

      {horizontalScroll ? (
        <div
          ref={scrollRef}
          onScroll={checkScroll}
          className="flex gap-3 overflow-x-auto pb-2"
          style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
        >
          {children}
        </div>
      ) : (
        <div className="w-full">{children}</div>
      )}

      {seeAllLink && (
        <a href={seeAllLink} className="group mt-1 flex items-center text-[13px] font-semibold text-[#60A5FA] hover:text-[#93C5FD] transition-colors">
          {seeAllText}
          <ChevronRight className="ml-0.5 h-4 w-4 transition-transform duration-200 group-hover:translate-x-1" />
        </a>
      )}
    </div>
  );
};
