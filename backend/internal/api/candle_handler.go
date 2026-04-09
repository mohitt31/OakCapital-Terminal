package api

import (
	"net/http"
	"strconv"

	"synthbull/internal/db"

	"github.com/gin-gonic/gin"
)

// CandleHandler serves historical candle data from PostgreSQL.
type CandleHandler struct {
	repo *db.CandleRepo
}

func NewCandleHandler(repo *db.CandleRepo) *CandleHandler {
	return &CandleHandler{repo: repo}
}

// GetCandles handles GET /api/v1/candles?symbol=X&interval=1s&limit=500&before=<unix_ts>
func (h *CandleHandler) GetCandles(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	interval := c.DefaultQuery("interval", "1s")

	limit := 500
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 2000 {
		limit = 2000
	}

	var before int64
	if b := c.Query("before"); b != "" {
		if parsed, err := strconv.ParseInt(b, 10, 64); err == nil && parsed > 0 {
			before = parsed
		}
	}

	rows, err := h.repo.QueryCandles(c.Request.Context(), symbol, interval, limit, before)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rows)
}
