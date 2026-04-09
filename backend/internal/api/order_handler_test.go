package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"synthbull/internal/market"
	"synthbull/internal/portfolio"

	"github.com/gin-gonic/gin"
)

// testAuthMiddleware injects a fake user_id for testing
func testAuthMiddleware(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}
}

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mktMgr := market.NewManager()
	portMgr := portfolio.NewManager(nil, nil)

	ctx := context.Background()
	userID := "test-user"
	portMgr.LoadOrFetch(ctx, userID) // Creates default portfolio with 100,000.00 cash

	// Use BookManager so engine access goes through the mutex+broadcast wrapper.
	bm := NewBookManager(nil) // nil broadcaster is fine for tests
	t.Cleanup(bm.Close)

	cfg := market.Config{InitialPrice: 100, TickSize: 1, MinQty: 1}
	for _, sym := range []string{"RELIANCE", "TCS", "HDFCBANK"} {
		managed := bm.GetOrCreate(sym)
		_ = mktMgr.AddSymbolWithHandle(sym, market.Stock, cfg, managed.GetHandle(), managed.GetMu())
	}

	orderHandler := NewOrderHandler(bm, mktMgr, portMgr, nil, nil, NewLimitOrderTracker())

	r := gin.New()
	protected := r.Group("/")
	protected.Use(testAuthMiddleware(userID))

	protected.POST("/api/v1/order/limit/add", orderHandler.PlaceLimitOrder)
	protected.POST("/api/v1/order/market", orderHandler.PlaceMarketOrder)
	protected.POST("/api/v1/order/limit/cancel", orderHandler.CancelLimit)
	protected.POST("/api/v1/order/stop/add", orderHandler.AddStop)
	protected.POST("/api/v1/order/stop-limit/add", orderHandler.AddStopLimit)
	protected.POST("/api/v1/order/limit/modify", orderHandler.ModifyLimit)

	// Book endpoints are GET with query params.
	r.GET("/api/v1/book/info", orderHandler.GetBookInfo)
	r.GET("/api/v1/book/depth", orderHandler.GetBookDepth)

	return httptest.NewServer(r)
}

func postJSON(t *testing.T, client *http.Client, url string, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func postJSONWithStatus(t *testing.T, client *http.Client, url string, body map[string]interface{}) (int, map[string]interface{}) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func getJSON(t *testing.T, client *http.Client, url string) map[string]interface{} {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func TestAddLimitAndBookInfo(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	addResp := postJSON(t, srv.Client(), srv.URL+"/api/v1/order/limit/add", map[string]interface{}{
		"symbol":      "RELIANCE",
		"side":        "buy",
		"quantity":    10,
		"limit_price": 100,
	})
	if addResp["success"] != true {
		t.Fatalf("add limit failed: %+v", addResp)
	}
	if id, _ := addResp["order_id"].(float64); id <= 0 {
		t.Fatalf("invalid order_id = %v", addResp["order_id"])
	}

	bookResp := getJSON(t, srv.Client(), fmt.Sprintf("%s/api/v1/book/info?symbol=RELIANCE", srv.URL))
	if bookResp["success"] != true {
		t.Fatalf("book info failed: %+v", bookResp)
	}
	data, ok := bookResp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("book info data type = %T, want object", bookResp["data"])
	}
	if int(data["best_bid"].(float64)) != 100 {
		t.Fatalf("best_bid = %v, want 100", data["best_bid"])
	}
}

func TestMarketOrderAndDepthEndpoint(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	_ = postJSON(t, srv.Client(), srv.URL+"/api/v1/order/limit/add", map[string]interface{}{
		"symbol":      "TCS",
		"side":        "sell",
		"quantity":    10,
		"limit_price": 100,
	})

	marketResp := postJSON(t, srv.Client(), srv.URL+"/api/v1/order/market", map[string]interface{}{
		"symbol":   "TCS",
		"side":     "buy",
		"quantity": 5,
	})
	if marketResp["success"] != true {
		t.Fatalf("market order failed: %+v", marketResp)
	}
	if id, _ := marketResp["order_id"].(float64); id <= 0 {
		t.Fatalf("invalid market order_id = %v", marketResp["order_id"])
	}

	depthResp := getJSON(t, srv.Client(), fmt.Sprintf("%s/api/v1/book/depth?symbol=TCS", srv.URL))
	if depthResp["success"] != true {
		t.Fatalf("book depth failed: %+v", depthResp)
	}
}

func TestMarketOrderInsufficientCashReturnsError(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	// Seed an expensive ask so a market buy would exceed the default 100,000 cash.
	addResp := postJSON(t, srv.Client(), srv.URL+"/api/v1/order/limit/add", map[string]interface{}{
		"symbol":      "TCS",
		"side":        "sell",
		"quantity":    1,
		"limit_price": 20000000, // 200,000.00
	})
	if addResp["success"] != true {
		t.Fatalf("seed ask failed: %+v", addResp)
	}

	status, marketResp := postJSONWithStatus(t, srv.Client(), srv.URL+"/api/v1/order/market", map[string]interface{}{
		"symbol":   "TCS",
		"side":     "buy",
		"quantity": 1,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (resp=%+v)", status, http.StatusBadRequest, marketResp)
	}
	if marketResp["success"] == true {
		t.Fatalf("expected unsuccessful response, got success: %+v", marketResp)
	}
	errStr, _ := marketResp["error"].(string)
	if !strings.Contains(strings.ToLower(errStr), "insufficient") {
		t.Fatalf("expected insufficient funds error, got: %+v", marketResp)
	}
}

func TestCancelLimitNotFound(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	addResp := postJSON(t, srv.Client(), srv.URL+"/api/v1/order/limit/add", map[string]interface{}{
		"symbol":      "HDFCBANK",
		"side":        "buy",
		"quantity":    3,
		"limit_price": 45,
	})
	if addResp["success"] != true {
		t.Fatalf("add limit failed: %+v", addResp)
	}

	// Cancel with a bogus order ID — should fail.
	cancelResp := postJSON(t, srv.Client(), srv.URL+"/api/v1/order/limit/cancel", map[string]interface{}{
		"symbol":   "HDFCBANK",
		"order_id": 99999,
	})
	if cancelResp["success"] == true {
		t.Fatalf("expected cancel to fail, got success: %+v", cancelResp)
	}
	errStr, _ := cancelResp["error"].(string)
	if errStr == "" {
		t.Fatalf("expected error string in response, got: %+v", cancelResp)
	}
}
