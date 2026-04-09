package api

import (
	"net/http"
	"synthbull/internal/market"

	"github.com/gin-gonic/gin"
)

// MarketHandler handles API requests for managing the market simulation.
type MarketHandler struct {
	marketManager *market.Manager
}

// NewMarketHandler creates a new handler for market operations.
func NewMarketHandler(manager *market.Manager) *MarketHandler {
	return &MarketHandler{marketManager: manager}
}

// ListSymbols handles requests to list all active symbols in the simulation.
func (h *MarketHandler) ListSymbols(c *gin.Context) {
	symbols := h.marketManager.ListSymbols()
	c.JSON(http.StatusOK, gin.H{"symbols": symbols})
}

// GetSymbolDetails handles requests for details about a specific symbol.
func (h *MarketHandler) GetSymbolDetails(c *gin.Context) {
	symbol := c.Param("symbol")
	info := h.marketManager.GetSymbol(symbol)
	if info == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Symbol not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":    info.Symbol,
		"class":     info.Class,
		"generator": info.Generator.GetConfig(),
	})
}

// StartSymbol handles requests to start the simulation for a specific symbol.
func (h *MarketHandler) StartSymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	info := h.marketManager.GetSymbol(symbol)
	if info == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Symbol not found"})
		return
	}

	info.Generator.Start()
	c.JSON(http.StatusOK, gin.H{"status": "started", "symbol": symbol})
}

// StopSymbol handles requests to stop the simulation for a specific symbol.
func (h *MarketHandler) StopSymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	info := h.marketManager.GetSymbol(symbol)
	if info == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Symbol not found"})
		return
	}

	info.Generator.Stop()
	c.JSON(http.StatusOK, gin.H{"status": "stopped", "symbol": symbol})
}
