import { useEffect, useRef, useState } from 'react'
import type { MarketFeed } from './useMockMarket'
import type { MarketSnapshot } from '../types/market'

export type MarketTelemetry = {
  incomingRate: number
  flushRate: number
  pressure: number
}

const cloneSnapshot = (snapshot: MarketSnapshot): MarketSnapshot => ({
  lastPrice: snapshot.lastPrice,
  tickDirection: snapshot.tickDirection,
  bids: snapshot.bids.map((row) => ({ ...row })),
  asks: snapshot.asks.map((row) => ({ ...row })),
  candles1s: snapshot.candles1s.map((row) => ({ ...row })),
  candles5s: snapshot.candles5s.map((row) => ({ ...row })),
  cashBalance: snapshot.cashBalance,
  positions: snapshot.positions.map((row) => ({ ...row })),
  fills: snapshot.fills.map((row) => ({ ...row })),
  trendingStocks: snapshot.trendingStocks.map((row) => ({ ...row })),
})

export const useMarketData = (feed: MarketFeed, flushMs = 100) => {
  const [snapshot, setSnapshot] = useState<MarketSnapshot>(() => feed.getSnapshot())
  const [telemetry, setTelemetry] = useState<MarketTelemetry>({
    incomingRate: 0,
    flushRate: 0,
    pressure: 0,
  })
  const latestRef = useRef<MarketSnapshot>(snapshot)
  const incomingCountRef = useRef(0)
  const flushCountRef = useRef(0)

  useEffect(() => {
    latestRef.current = cloneSnapshot(feed.getSnapshot())
    const unsubscribe = feed.subscribe((next) => {
      incomingCountRef.current += 1
      latestRef.current = next
    })
    const flush = setInterval(() => {
      flushCountRef.current += 1
      setSnapshot(cloneSnapshot(latestRef.current))
    }, flushMs)
    const throughput = setInterval(() => {
      const incomingRate = incomingCountRef.current
      const flushRate = flushCountRef.current
      setTelemetry({
        incomingRate,
        flushRate,
        pressure: Math.max(0, incomingRate - flushRate),
      })
      incomingCountRef.current = 0
      flushCountRef.current = 0
    }, 1000)
    return () => {
      unsubscribe()
      clearInterval(flush)
      clearInterval(throughput)
    }
  }, [feed, flushMs])

  return { snapshot, telemetry }
}
