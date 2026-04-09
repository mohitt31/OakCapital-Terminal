export interface StockData {
  symbol: string
  name: string
  price: number
  currency: string
  changePercent: number
  changeValue?: number
  isDerived?: boolean
}

export const indianStocks: StockData[] = [
  { symbol: 'RELIANCE', name: 'Reliance Industries', price: 1411.80, currency: 'INR', changePercent: 0.28 },
  { symbol: 'TCS', name: 'Tata Consultancy Services', price: 2398.80, currency: 'INR', changePercent: 0.63 },
  { symbol: 'HDFCBANK', name: 'HDFC Bank', price: 764.90, currency: 'INR', changePercent: 2.79 },
  { symbol: 'ICICIBANK', name: 'ICICI Bank', price: 1251.20, currency: 'INR', changePercent: 2.33 },
]

export const indices: StockData[] = [
  { symbol: 'NIFTY50', name: 'Nifty 50', price: 22912.40, currency: 'INR', changePercent: 1.78 },
  { symbol: 'SENSEX', name: 'Sensex', price: 74068.45, currency: 'INR', changePercent: 1.89, isDerived: true },
  { symbol: 'NIFTY500', name: 'Nifty 500', price: 21067.00, currency: 'INR', changePercent: 1.95 },
  { symbol: 'NIFTYMIDCAP', name: 'Nifty MidCap', price: 54087.00, currency: 'INR', changePercent: 2.60 },
]

export const worldIndices: StockData[] = [
  { symbol: 'SPX', name: 'S&P 500 Index', price: 6576.71, currency: 'USD', changePercent: -0.07 },
  { symbol: 'NDX', name: 'Nasdaq 100 Index', price: 24074.79, currency: 'USD', changePercent: -0.47, isDerived: true },
  { symbol: 'DJI', name: 'Dow Jones Industrial Avera...', price: 46278.50, currency: 'USD', changePercent: 0.15 },
  { symbol: 'NI225', name: 'Japan 225 Index', price: 52252.21, currency: 'JPY', changePercent: 1.43 },
  { symbol: 'UKX', name: 'FTSE 100 Index', price: 9965.16, currency: 'GBP', changePercent: 0.72, isDerived: true },
]

export const communityTrends: StockData[] = [
  { symbol: 'COALINDIA', name: 'Coal India Ltd.', price: 442.10, currency: 'INR', changePercent: -2.89 },
  { symbol: 'APOLLOHOSP', name: 'Apollo Hospitals Enterprise...', price: 7413.00, currency: 'INR', changePercent: 3.75 },
  { symbol: 'ETERNAL', name: 'Eternal Limited', price: 237.94, currency: 'INR', changePercent: 4.84 },
  { symbol: 'ASIANPAINT', name: 'Asian Paints Ltd.', price: 2217.30, currency: 'INR', changePercent: 4.53 },
  { symbol: 'LT', name: 'Larsen & Toubro Limited', price: 3516.80, currency: 'INR', changePercent: 5.22 },
]

export const highestVolume: StockData[] = [
  { symbol: 'HDFCBANK', name: 'HDFC Bank Limited', price: 764.90, currency: 'INR', changePercent: 2.79 },
  { symbol: 'SBIN', name: 'State Bank of India', price: 1030.80, currency: 'INR', changePercent: -0.11 },
  { symbol: 'ICICIBANK', name: 'ICICI Bank Limited', price: 1251.20, currency: 'INR', changePercent: 2.33 },
  { symbol: 'GUJALKALI', name: 'Gujarat Alkalies & C...', price: 625.95, currency: 'INR', changePercent: 1.76 },
  { symbol: 'RELIANCE', name: 'Reliance Industries...', price: 1411.80, currency: 'INR', changePercent: 0.28 },
  { symbol: 'BLS', name: 'BLS International Ser...', price: 259.40, currency: 'INR', changePercent: 16.98 },
]

export const mostVolatile: StockData[] = [
  { symbol: 'TECHNICHEM', name: 'Technichem Organics...', price: 45.00, currency: 'INR', changePercent: 7.40, isDerived: true },
  { symbol: 'VJTFEDU', name: 'VJTF Eduservices Ltd.', price: 93.04, currency: 'INR', changePercent: 9.72, isDerived: true },
  { symbol: 'MTPL', name: 'Marg Techno Project...', price: 24.37, currency: 'INR', changePercent: -13.18, isDerived: true },
  { symbol: 'PRIMEPRO', name: 'Prime Property Deve...', price: 17.44, currency: 'INR', changePercent: -4.75, isDerived: true },
  { symbol: 'RAJMET', name: 'Rajnandini Metal Ltd.', price: 3.10, currency: 'INR', changePercent: -7.46 },
  { symbol: 'ULTRACAB', name: 'Ultracab (India) Ltd', price: 6.16, currency: 'INR', changePercent: 0.65, isDerived: true },
]

export const gainers: StockData[] = [
  { symbol: 'PRARUH', name: 'Praruh Technologies Limited', price: 55.20, currency: 'INR', changePercent: 20.00, isDerived: true },
  { symbol: 'AMTL', name: 'Advance Metering Technology Ltd.', price: 16.26, currency: 'INR', changePercent: 20.00, isDerived: true },
  { symbol: 'KKSHL', name: 'KK Shah Hospitals Limited', price: 39.60, currency: 'INR', changePercent: 20.00, isDerived: true },
  { symbol: 'EPIC', name: 'Epic Energy Limited', price: 35.74, currency: 'INR', changePercent: 19.97, isDerived: true },
  { symbol: 'MEGACOR', name: 'Mega Corporation Limited', price: 2.44, currency: 'INR', changePercent: 19.61, isDerived: true },
  { symbol: 'NINSYS', name: 'NINtec Systems Ltd', price: 342.15, currency: 'INR', changePercent: 19.38 },
]

export const losers: StockData[] = [
  { symbol: 'FINOPB', name: 'FINO Payments Bank Ltd.', price: 112.46, currency: 'INR', changePercent: -20.00 },
  { symbol: 'GICL', name: 'Globe International Carriers Ltd.', price: 31.36, currency: 'INR', changePercent: -19.86 },
  { symbol: 'EVERESTO', name: 'Everest Organics Ltd.', price: 226.00, currency: 'INR', changePercent: -19.57, isDerived: true },
  { symbol: 'FLEXFO', name: 'Flex Foods Limited', price: 30.57, currency: 'INR', changePercent: -17.22, isDerived: true },
  { symbol: 'STML', name: 'Steelman Telecom Ltd.', price: 53.89, currency: 'INR', changePercent: -15.80, isDerived: true },
  { symbol: 'GOLDLEG', name: 'Golden Legand Leasing & Finance Ltd', price: 7.73, currency: 'INR', changePercent: -15.05, isDerived: true },
]

const universeSource = [
  ...indianStocks,
  ...indices,
  ...worldIndices,
  ...communityTrends,
  ...highestVolume,
  ...mostVolatile,
  ...gainers,
  ...losers,
]

const uniqueUniverse = new Map<string, StockData>()
universeSource.forEach((item) => {
  if (!uniqueUniverse.has(item.symbol)) uniqueUniverse.set(item.symbol, item)
})

export const terminalUniverse = Array.from(uniqueUniverse.values()).map((item) => ({
  symbol: item.symbol,
  price: item.price,
}))
