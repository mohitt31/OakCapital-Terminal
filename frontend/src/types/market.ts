export type Side = 'bid' | 'ask'

export type CandlePoint = {
  time: number
  open: number
  high: number
  low: number
  close: number
}

export type BookLevel = {
  price: number
  quantity: number
  cumulative: number
  flashUntil: number
}

export type PortfolioPosition = {
  asset: string
  quantity: number
  entryPrice: number
  markPrice: number
  pnl: number
}

export type TradeFill = {
  id: number
  asset: string
  action: TradeAction
  direction: 'long' | 'short'
  quantity: number
  price: number
  timestamp: number
}

export type TrendingStock = {
  symbol: string
  price: number
  changePct: number
  volume: number
}

export type MarketSnapshot = {
  lastPrice: number
  tickDirection: 1 | -1 | 0
  bids: BookLevel[]
  asks: BookLevel[]
  candles1s: CandlePoint[]
  candles5s: CandlePoint[]
  cashBalance: number
  positions: PortfolioPosition[]
  fills: TradeFill[]
  trendingStocks: TrendingStock[]
}

export type TradeAction = 'buy' | 'sell'

export type TradeRequest = {
  asset: string
  action: TradeAction
  direction: 'long' | 'short'
  quantity: number
  orderType: 'limit' | 'market'
  limitPrice: number
  timestamp: number
}
