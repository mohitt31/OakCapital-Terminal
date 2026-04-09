import { useMemo, useState } from 'react'
import type { TradeAction, TradeRequest } from '../types/market'

type Direction = 'long' | 'short'

export type TradeIntent = {
  direction: Direction
  quantity: number
  limitPrice: number
  orderType: 'limit' | 'market'
  sideLabel: string
  canSubmit: boolean
}

export const useTradeControls = (marketPrice: number, asset: string, onExecute: (request: TradeRequest) => void) => {
  const [direction, setDirection] = useState<Direction>('long')
  const [orderType, setOrderType] = useState<'limit' | 'market'>('limit')
  const [quantityText, setQuantityText] = useState('0.010')
  const [priceText, setPriceText] = useState(() => marketPrice.toFixed(2))

  const quantity = Number(quantityText)
  const limitPrice = Number(priceText)
  const sideLabel = `${orderType.toUpperCase()} · ${direction === 'long' ? 'Buy to Open / Sell to Close' : 'Sell to Open / Buy to Close'}`
  const canSubmit =
    Number.isFinite(quantity) && quantity > 0 && (orderType === 'market' || (Number.isFinite(limitPrice) && limitPrice > 0))

  const intent = useMemo<TradeIntent>(
    () => ({
      direction,
      quantity: Number.isFinite(quantity) ? quantity : 0,
      limitPrice: orderType === 'market' ? marketPrice : Number.isFinite(limitPrice) ? limitPrice : 0,
      orderType,
      sideLabel,
      canSubmit,
    }),
    [direction, quantity, limitPrice, marketPrice, orderType, sideLabel, canSubmit],
  )

  const setLong = () => setDirection('long')
  const setShort = () => setDirection('short')
  const quickBuy = () => {
    setDirection('long')
    setPriceText(marketPrice.toFixed(2))
  }
  const quickSell = () => {
    setDirection('short')
    setPriceText(marketPrice.toFixed(2))
  }

  const execute = (action: TradeAction) => {
    if (!canSubmit) return
    const payload: TradeRequest = {
      asset,
      action,
      direction: intent.direction,
      quantity: intent.quantity,
      orderType,
      limitPrice: intent.limitPrice,
      timestamp: Date.now(),
    }
    onExecute(payload)
  }

  return {
    direction,
    orderType,
    quantityText,
    priceText,
    intent,
    setQuantityText,
    setPriceText,
    setOrderType,
    setLong,
    setShort,
    quickBuy,
    quickSell,
    execute,
  }
}
