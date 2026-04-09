package api

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"synthbull/internal/auth"
	"synthbull/internal/db"
	"synthbull/internal/engine"
	"synthbull/internal/eventbus"
	"synthbull/internal/market"
	"synthbull/internal/portfolio"
	"synthbull/pkg/models"

	"github.com/gin-gonic/gin"
)

// OrderHandler handles all order and book REST endpoints.
type OrderHandler struct {
	bookManager  *BookManager
	mktMgr       *market.Manager // symbol validation only
	portfolioMgr *portfolio.Manager
	authSvc      *auth.Service
	bus          *eventbus.Publisher
	limitTracker *LimitOrderTracker
}

// NewOrderHandler creates an OrderHandler wired to the shared BookManager, market
// manager (for symbol validation), portfolio manager, auth service, and optional
// Redis publisher.
func NewOrderHandler(
	bm *BookManager,
	mktMgr *market.Manager,
	mgr *portfolio.Manager,
	authSvc *auth.Service,
	bus *eventbus.Publisher,
	limitTracker *LimitOrderTracker,
) *OrderHandler {
	return &OrderHandler{
		bookManager:  bm,
		mktMgr:       mktMgr,
		portfolioMgr: mgr,
		authSvc:      authSvc,
		bus:          bus,
		limitTracker: limitTracker,
	}
}

// Health returns a simple liveness check with the list of active order books.
func (h *OrderHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":       "healthy",
		"active_books": h.bookManager.ListSymbols(),
		"timestamp":    models.Now(),
	})
}

// ---------------------------------------------------------------------------
// Order placement
// ---------------------------------------------------------------------------

// PlaceLimitOrder handles POST /api/v1/order/limit/add
func (h *OrderHandler) PlaceLimitOrder(c *gin.Context) {
	var req AddLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	side, valid := parseSide(string(req.Side))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid side: must be 'buy' or 'sell'"})
		return
	}

	if h.mktMgr.GetSymbol(req.Symbol) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown symbol"})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	realPrice := float64(req.LimitPrice) / 100.0
	realQty := float64(req.Quantity)

	// Reserve cash (escrow) before touching the engine.
	if strings.ToLower(string(req.Side)) == "buy" {
		if err := h.portfolioMgr.ReserveCash(c.Request.Context(), userID, realPrice, realQty); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient funds: " + err.Error()})
			return
		}
	}

	book := h.bookManager.GetOrCreate(req.Symbol)
	orderID, status, result := book.AddLimitAutoID(side, int(req.Quantity), int(req.LimitPrice))
	if status != engine.StatusOK {
		// Refund escrow on engine failure.
		if strings.ToLower(string(req.Side)) == "buy" && h.portfolioMgr != nil {
			h.portfolioMgr.ReleaseCash(c.Request.Context(), userID, realPrice, realQty)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine error: " + status.Error()})
		return
	}

	if h.limitTracker != nil {
		h.limitTracker.Track(userID, req.Symbol, req.Side, int64(orderID), req.Quantity, req.LimitPrice)
	}

	// Apply immediate fills to the portfolio.
	filledQty := int64(0)
	var totalCost int64
	if len(result.Trades) > 0 && h.portfolioMgr != nil {
		for _, t := range result.Trades {
			fillPrice := float64(t.Price) / 100.0
			fillQty := float64(t.Qty)
			if _, err := h.portfolioMgr.ApplyFill(c.Request.Context(), userID, req.Symbol, strings.ToLower(string(req.Side)), fillPrice, fillQty, true); err != nil {
				log.Printf("[order] ApplyFill error for user %s, symbol %s: %v", userID, req.Symbol, err)
			}
			filledQty += int64(t.Qty)
			totalCost += int64(t.Price) * int64(t.Qty)
		}
		h.portfolioMgr.UpdateAllMarkPrices(req.Symbol, float64(result.Trades[len(result.Trades)-1].Price)/100.0)
	}

	if h.limitTracker != nil && len(result.Trades) > 0 {
		releaseReserve, completed := h.limitTracker.RecordPlacementTrades(req.Symbol, int64(orderID), result.Trades)
		if completed && releaseReserve > 0 && h.portfolioMgr != nil {
			h.portfolioMgr.ReleaseCashAmount(c.Request.Context(), userID, releaseReserve)
		}
	}

	// Publish order update to Redis stream for DBWriter.
	if h.bus != nil {
		orderStatus := models.StatusOpen
		if filledQty >= int64(req.Quantity) {
			orderStatus = models.StatusFilled
		} else if filledQty > 0 {
			orderStatus = models.StatusPartiallyFilled
		}
		avgPrice := int64(0)
		if filledQty > 0 {
			avgPrice = totalCost / filledQty
		}
		_ = h.bus.PublishOrderUpdate(context.Background(), eventbus.OrderUpdateEvent{
			OrderID:      int64(orderID),
			UserID:       userID,
			Symbol:       req.Symbol,
			Side:         req.Side,
			Type:         models.TypeLimit,
			Status:       orderStatus,
			Quantity:     int64(req.Quantity),
			FilledQty:    filledQty,
			RemainingQty: int64(req.Quantity) - filledQty,
			AvgPrice:     avgPrice,
			LimitPrice:   int64(req.LimitPrice),
			UpdatedAt:    time.Now().UTC(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "limit order placed",
		"order_id": orderID,
		"symbol":   req.Symbol,
		"side":     req.Side,
		"quantity": req.Quantity,
		"price":    req.LimitPrice,
		"trades":   result.Trades,
	})
}

// PlaceMarketOrder handles POST /api/v1/order/market
func (h *OrderHandler) PlaceMarketOrder(c *gin.Context) {
	var req MarketOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	side, valid := parseSide(string(req.Side))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid side: must be 'buy' or 'sell'"})
		return
	}

	if h.mktMgr.GetSymbol(req.Symbol) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown symbol"})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	book := h.bookManager.GetOrCreate(req.Symbol)

	// Pre-execution validation: engine fills cannot be rolled back.
	if h.portfolioMgr != nil {
		snap, err := h.portfolioMgr.GetSnapshot(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "portfolio unavailable: " + err.Error()})
			return
		}
		switch side {
		case engine.SideBuy:
			estCostCents, fillableQty := estimateMarketBuyCost(book.GetDepth(), req.Quantity)
			if fillableQty == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "no ask liquidity available for market buy"})
				return
			}
			requiredCash := float64(estCostCents) / 100.0
			if snap.AvailableCash+1e-9 < requiredCash {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":          fmt.Sprintf("insufficient available cash for market buy: need %.2f, available %.2f", requiredCash, snap.AvailableCash),
					"available_cash": snap.AvailableCash,
					"estimated_cost": requiredCash,
				})
				return
			}
		case engine.SideSell:
			var avail float64
			for i := range snap.Positions {
				if snap.Positions[i].Symbol == req.Symbol {
					avail = snap.Positions[i].Quantity
					break
				}
			}
			if avail < float64(req.Quantity) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":            "insufficient assets",
					"available_shares": avail,
					"required_shares":  req.Quantity,
				})
				return
			}
		}
	}

	orderID, status, result := book.MarketAutoID(side, int(req.Quantity))
	if status != engine.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine error: " + status.Error()})
		return
	}

	filledQty := int64(0)
	var totalCostMkt int64
	if len(result.Trades) > 0 && h.portfolioMgr != nil {
		for _, t := range result.Trades {
			if _, err := h.portfolioMgr.ApplyFill(c.Request.Context(), userID, req.Symbol, strings.ToLower(string(req.Side)), float64(t.Price)/100.0, float64(t.Qty), false); err != nil {
				log.Printf("[order] ApplyFill error for user %s, symbol %s: %v", userID, req.Symbol, err)
			}
			filledQty += int64(t.Qty)
			totalCostMkt += int64(t.Price) * int64(t.Qty)
		}
		h.portfolioMgr.UpdateAllMarkPrices(req.Symbol, float64(result.Trades[len(result.Trades)-1].Price)/100.0)
	}

	if h.bus != nil {
		orderStatus := models.StatusFilled
		if filledQty < int64(req.Quantity) {
			orderStatus = models.StatusPartiallyFilled
		}
		avgPrice := int64(0)
		if filledQty > 0 {
			avgPrice = totalCostMkt / filledQty
		}
		_ = h.bus.PublishOrderUpdate(context.Background(), eventbus.OrderUpdateEvent{
			OrderID:      int64(orderID),
			UserID:       userID,
			Symbol:       req.Symbol,
			Side:         req.Side,
			Type:         models.TypeMarket,
			Status:       orderStatus,
			Quantity:     int64(req.Quantity),
			FilledQty:    filledQty,
			RemainingQty: int64(req.Quantity) - filledQty,
			AvgPrice:     avgPrice,
			UpdatedAt:    time.Now().UTC(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "market order executed",
		"order_id": orderID,
		"symbol":   req.Symbol,
		"side":     req.Side,
		"quantity": req.Quantity,
		"trades":   result.Trades,
	})
}

// PlaceInternalMarketOrder handles POST /internal/simbot/order/market
// Loopback-only endpoint used by in-process simulation bots.
func (h *OrderHandler) PlaceInternalMarketOrder(c *gin.Context) {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid remote address"})
		return
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "internal endpoint is loopback-only"})
		return
	}

	type internalMarketOrderRequest struct {
		Symbol   string      `json:"symbol" binding:"required"`
		Side     models.Side `json:"side" binding:"required,oneof=buy sell"`
		Quantity int64       `json:"quantity" binding:"required,gt=0"`
		Mode     string      `json:"mode,omitempty"`
		UserID   string      `json:"user_id,omitempty"`
	}
	var req internalMarketOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	side, valid := parseSide(string(req.Side))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid side: must be 'buy' or 'sell'"})
		return
	}
	if h.mktMgr.GetSymbol(req.Symbol) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown symbol"})
		return
	}

	book := h.bookManager.GetOrCreate(req.Symbol)
	orderID, status, result := book.MarketAutoID(side, int(req.Quantity))
	if status != engine.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine error: " + status.Error()})
		return
	}

	filledQty := int64(0)
	var totalCost int64
	for _, t := range result.Trades {
		filledQty += int64(t.Qty)
		totalCost += int64(t.Price) * int64(t.Qty)
	}
	orderStatus := models.StatusFilled
	if filledQty < int64(req.Quantity) {
		orderStatus = models.StatusPartiallyFilled
	}
	avgPrice := int64(0)
	if filledQty > 0 {
		avgPrice = totalCost / filledQty
	}

	// Apply fills to the portfolio for any authenticated user (simulation AND live).
	// Previously this was gated on mode=="live", which meant simulation-bot trades
	// never updated the user's cash or positions. Now both modes update the portfolio
	// so the user sees their equity change in real time.
	if req.UserID != "" && len(result.Trades) > 0 && h.portfolioMgr != nil {
		for _, t := range result.Trades {
			if _, err := h.portfolioMgr.ApplyFill(c.Request.Context(), req.UserID, req.Symbol, strings.ToLower(string(req.Side)), float64(t.Price)/100.0, float64(t.Qty), false); err != nil {
				log.Printf("[order/internal] ApplyFill error for user %s, symbol %s: %v", req.UserID, req.Symbol, err)
			}
		}
		h.portfolioMgr.UpdateAllMarkPrices(req.Symbol, float64(result.Trades[len(result.Trades)-1].Price)/100.0)
	}

	if h.bus != nil {
		targetUserID := "internal-bot"
		if req.Mode == "live" && req.UserID != "" {
			targetUserID = req.UserID
		}
		_ = h.bus.PublishOrderUpdate(context.Background(), eventbus.OrderUpdateEvent{
			OrderID:      int64(orderID),
			UserID:       targetUserID,
			Symbol:       req.Symbol,
			Side:         req.Side,
			Type:         models.TypeMarket,
			Status:       orderStatus,
			Quantity:     int64(req.Quantity),
			FilledQty:    filledQty,
			RemainingQty: int64(req.Quantity) - filledQty,
			AvgPrice:     avgPrice,
			UpdatedAt:    time.Now().UTC(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "internal market order executed",
		"order_id": orderID,
		"symbol":   req.Symbol,
		"side":     req.Side,
		"quantity": req.Quantity,
		"mode":     req.Mode,
		"trades":   result.Trades,
	})
}

// ---------------------------------------------------------------------------
// Limit order lifecycle
// ---------------------------------------------------------------------------

// CancelLimit handles POST /api/v1/order/limit/cancel
func (h *OrderHandler) CancelLimit(c *gin.Context) {
	var req CancelLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	book := h.bookManager.Get(req.Symbol)
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no order book for symbol"})
		return
	}

	status, _ := book.CancelLimit(int(req.OrderID))
	if status != engine.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cancel failed: " + status.Error()})
		return
	}

	if h.limitTracker != nil {
		if tracked, ok := h.limitTracker.Remove(req.Symbol, req.OrderID); ok {
			if tracked.Side == models.SideBuy && tracked.RemainingReserve > 0 && h.portfolioMgr != nil {
				h.portfolioMgr.ReleaseCashAmount(c.Request.Context(), tracked.UserID, tracked.RemainingReserve)
			}
		}
	}

	// Publish cancelled status.
	userID := c.GetString("user_id")
	if h.bus != nil && userID != "" {
		_ = h.bus.PublishOrderUpdate(context.Background(), eventbus.OrderUpdateEvent{
			OrderID:   int64(req.OrderID),
			UserID:    userID,
			Symbol:    req.Symbol,
			Type:      models.TypeLimit,
			Status:    models.StatusCancelled,
			UpdatedAt: time.Now().UTC(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": req.OrderID, "symbol": req.Symbol})
}

// ModifyLimit handles POST /api/v1/order/limit/modify
func (h *OrderHandler) ModifyLimit(c *gin.Context) {
	var req ModifyLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	book := h.bookManager.Get(req.Symbol)
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no order book for symbol"})
		return
	}

	status, result := book.ModifyLimit(int(req.OrderID), int(req.Quantity), int(req.LimitPrice))
	if status != engine.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "modify failed: " + status.Error()})
		return
	}

	if h.limitTracker != nil {
		h.limitTracker.UpdateLimit(req.Symbol, req.OrderID, req.Quantity, req.LimitPrice)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": req.OrderID, "trades": result.Trades})
}

// ListOrders handles GET /api/v1/orders
// Returns all orders for the user (open and history) by fetching the latest status of each order from order_history.
func (h *OrderHandler) ListOpenOrders(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if db.Pool == nil {
		c.JSON(http.StatusOK, gin.H{"orders": []interface{}{}})
		return
	}

	query := `
		SELECT order_id, symbol, side, order_type, status, quantity, filled_qty, avg_price, limit_price, updated_at
		FROM (
			SELECT DISTINCT ON (symbol, order_id)
				order_id, symbol, side, order_type, status, quantity, filled_qty, avg_price, limit_price, updated_at
			FROM order_history
			WHERE user_id = $1
			ORDER BY symbol, order_id, updated_at DESC
		) latest
		ORDER BY updated_at DESC
	`
	rows, err := db.Pool.Query(c.Request.Context(), query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query orders: " + err.Error()})
		return
	}
	defer rows.Close()

	type OrderResponse struct {
		ID         int64   `json:"id"`
		Asset      string  `json:"asset"`
		Action     string  `json:"action"`
		OrderType  string  `json:"orderType"`
		Status     string  `json:"status"`
		Quantity   float64 `json:"quantity"`
		FilledQty  float64 `json:"filled_qty"`
		AvgPrice   float64 `json:"avgPrice"`
		LimitPrice float64 `json:"limitPrice"`
		Timestamp  int64   `json:"timestamp"`
	}

	var orders []OrderResponse
	for rows.Next() {
		var o OrderResponse
		var qty, filledQty, avgPrice, limitPrice float64
		var updatedAt time.Time

		err := rows.Scan(&o.ID, &o.Asset, &o.Action, &o.OrderType, &o.Status, &qty, &filledQty, &avgPrice, &limitPrice, &updatedAt)
		if err != nil {
			continue
		}

		o.Quantity = qty
		o.FilledQty = filledQty
		o.AvgPrice = avgPrice
		o.LimitPrice = limitPrice
		o.Timestamp = updatedAt.UnixNano() / int64(time.Millisecond)

		orders = append(orders, o)
	}

	if orders == nil {
		orders = []OrderResponse{}
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

// ---------------------------------------------------------------------------
// Stop orders
// ---------------------------------------------------------------------------

// AddStop handles POST /api/v1/order/stop/add
func (h *OrderHandler) AddStop(c *gin.Context) {
	var req AddStopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	side, valid := parseSide(string(req.Side))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid side"})
		return
	}

	if h.mktMgr.GetSymbol(req.Symbol) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown symbol"})
		return
	}

	book := h.bookManager.GetOrCreate(req.Symbol)
	orderID, status, _ := book.AddStopAutoID(side, int(req.Quantity), int(req.StopPrice))
	if status != engine.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine error: " + status.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": orderID})
}

// CancelStop handles POST /api/v1/order/stop/cancel
func (h *OrderHandler) CancelStop(c *gin.Context) {
	var req CancelStopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	book := h.bookManager.Get(req.Symbol)
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no order book for symbol"})
		return
	}

	status, _ := book.CancelStop(int(req.OrderID))
	if status != engine.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cancel failed: " + status.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": req.OrderID})
}

// ModifyStop handles POST /api/v1/order/stop/modify
func (h *OrderHandler) ModifyStop(c *gin.Context) {
	var req ModifyStopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	book := h.bookManager.Get(req.Symbol)
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no order book for symbol"})
		return
	}

	status, result := book.ModifyStop(int(req.OrderID), int(req.Quantity), int(req.StopPrice))
	if status != engine.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "modify failed: " + status.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": req.OrderID, "trades": result.Trades})
}

// ---------------------------------------------------------------------------
// Stop-limit orders
// ---------------------------------------------------------------------------

// AddStopLimit handles POST /api/v1/order/stop-limit/add
func (h *OrderHandler) AddStopLimit(c *gin.Context) {
	var req AddStopLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	side, valid := parseSide(string(req.Side))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid side"})
		return
	}

	if h.mktMgr.GetSymbol(req.Symbol) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown symbol"})
		return
	}

	book := h.bookManager.GetOrCreate(req.Symbol)
	orderID, status, _ := book.AddStopLimitAutoID(side, int(req.Quantity), int(req.LimitPrice), int(req.StopPrice))
	if status != engine.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine error: " + status.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": orderID})
}

// CancelStopLimit handles POST /api/v1/order/stop-limit/cancel
func (h *OrderHandler) CancelStopLimit(c *gin.Context) {
	var req CancelStopLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	book := h.bookManager.Get(req.Symbol)
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no order book for symbol"})
		return
	}

	status, _ := book.CancelStopLimit(int(req.OrderID))
	if status != engine.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cancel failed: " + status.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": req.OrderID})
}

// ModifyStopLimit handles POST /api/v1/order/stop-limit/modify
func (h *OrderHandler) ModifyStopLimit(c *gin.Context) {
	var req ModifyStopLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	book := h.bookManager.Get(req.Symbol)
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no order book for symbol"})
		return
	}

	status, result := book.ModifyStopLimit(int(req.OrderID), int(req.Quantity), int(req.LimitPrice), int(req.StopPrice))
	if status != engine.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "modify failed: " + status.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order_id": req.OrderID, "trades": result.Trades})
}

// ---------------------------------------------------------------------------
// Book / market data
// ---------------------------------------------------------------------------

// GetBookInfo handles GET /api/v1/book/info?symbol=RELIANCE
func (h *OrderHandler) GetBookInfo(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol query param is required"})
		return
	}

	book := h.bookManager.Get(symbol)
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no order book for symbol"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": BookInfoResponse{
			Symbol:            symbol,
			BestBid:           int64(book.BestBid()),
			BestAsk:           int64(book.BestAsk()),
			LastExecutedPrice: int64(book.LastExecutedPrice()),
			Timestamp:         models.Now(),
		},
	})
}

// GetBookDepth handles GET /api/v1/book/depth?symbol=RELIANCE
func (h *OrderHandler) GetBookDepth(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol query param is required"})
		return
	}

	book := h.bookManager.Get(symbol)
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no order book for symbol"})
		return
	}

	depth := book.GetDepth()
	bids := make([]models.PriceLevel, len(depth.Bids))
	for i, b := range depth.Bids {
		bids[i] = models.PriceLevel{Price: int64(b.Price), Quantity: int64(b.Volume)}
	}
	asks := make([]models.PriceLevel, len(depth.Asks))
	for i, a := range depth.Asks {
		asks[i] = models.PriceLevel{Price: int64(a.Price), Quantity: int64(a.Volume)}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": BookDepthResponse{
			Symbol: symbol,
			Bids:   bids,
			Asks:   asks,
		},
	})
}

// ListBooks handles GET /api/v1/book/list
func (h *OrderHandler) ListBooks(c *gin.Context) {
	symbols := h.bookManager.ListSymbols()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"count": len(symbols), "symbols": symbols},
	})
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func parseSide(side string) (int, bool) {
	switch strings.ToLower(side) {
	case "buy":
		return engine.SideBuy, true
	case "sell":
		return engine.SideSell, true
	default:
		return 0, false
	}
}

func estimateMarketBuyCost(depth engine.Depth, requestedQty int64) (costCents int64, fillableQty int64) {
	remaining := requestedQty
	for _, level := range depth.Asks {
		if remaining <= 0 {
			break
		}
		if level.Price <= 0 || level.Volume <= 0 {
			continue
		}
		take := int64(level.Volume)
		if take > remaining {
			take = remaining
		}
		costCents += int64(level.Price) * take
		fillableQty += take
		remaining -= take
	}
	return costCents, fillableQty
}
