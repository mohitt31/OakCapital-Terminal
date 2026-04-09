package simbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// ──────────────────────────────────────────────────────────────────────────────
// ExchangeClient — WebSocket client to the exchange matching engine.
// Shared runtime client for builder and flagship simbot instances.
// ──────────────────────────────────────────────────────────────────────────────

type ExchangeClient struct {
	conn         *websocket.Conn
	writeMu      sync.Mutex
	subscribedTo string
}

// WsURL returns the exchange WebSocket URL (env override supported).
func WsURL() string {
	if url := os.Getenv("WS_URL"); url != "" {
		return url
	}
	return "ws://localhost:8080/ws/internal"
}

func internalOrderURL() string {
	if url := os.Getenv("INTERNAL_ORDER_URL"); url != "" {
		return url
	}
	return "http://localhost:8080/internal/simbot/order/market"
}

// Connect dials the exchange websocket.
func (c *ExchangeClient) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(WsURL(), nil)
	if err != nil {
		return fmt.Errorf("ws connect failed: %w", err)
	}
	c.conn = conn
	return nil
}

func (c *ExchangeClient) SubscribeTicker(symbol string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	msg := map[string]interface{}{
		"action":  "subscribe",
		"symbols": []string{symbol},
		// Subscribe to all events so price updates continue even when ticker
		// messages are sparse; candle updates provide a steady close stream.
		"event": "all",
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		return err
	}
	c.subscribedTo = symbol
	return nil
}

// Send marshals and writes an outgoing order.
func (c *ExchangeClient) Send(msg OutgoingOrder) error {
	qty := int64(math.Round(msg.Quantity))
	if qty <= 0 {
		return fmt.Errorf("quantity must be >= 1; got %.4f", msg.Quantity)
	}
	body := map[string]interface{}{
		"symbol":   msg.Symbol,
		"side":     msg.Side,
		"quantity": qty,
		"mode":     msg.Mode,
		"user_id":  msg.UserID,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, internalOrderURL(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		detail := strings.TrimSpace(string(payload))
		if detail != "" {
			return fmt.Errorf("internal order endpoint returned status %d: %s", resp.StatusCode, detail)
		}
		return fmt.Errorf("internal order endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

// Recv reads one JSON message from the websocket.
func (c *ExchangeClient) Recv() (IncomingMessage, error) {
	for {
		var msg IncomingMessage
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return msg, err
		}

		parts := strings.Split(string(data), "\n")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			type wsEnvelope struct {
				Type string                 `json:"type"`
				Data map[string]interface{} `json:"data"`
			}

			var env wsEnvelope
			if err := json.Unmarshal([]byte(part), &env); err != nil {
				continue
			}
			switch env.Type {
			case "ticker":
				if lp, ok := env.Data["last_price"].(float64); ok && lp > 0 {
					if ts, ok := env.Data["timestamp"].(float64); ok {
						msg.Timestamp = int64(ts)
					}
					msg.Prices = []Candle{{Close: lp / 100.0}}
					return msg, nil
				}
			case "candle":
				if closePx, ok := env.Data["close"].(float64); ok && closePx > 0 {
					openPx, _ := env.Data["open"].(float64)
					highPx, _ := env.Data["high"].(float64)
					lowPx, _ := env.Data["low"].(float64)
					msg.Prices = []Candle{{
						Open:  openPx / 100.0,
						High:  highPx / 100.0,
						Low:   lowPx / 100.0,
						Close: closePx / 100.0,
					}}
					if ts, ok := env.Data["timestamp"].(float64); ok {
						msg.Timestamp = int64(ts)
					}
					return msg, nil
				}
			}
		}
	}
}

// Close shuts down the websocket connection.
func (c *ExchangeClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
