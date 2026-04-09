package ws

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"synthbull/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development (restrict in production)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ClientMessage represents an incoming message from a WebSocket client.
type ClientMessage struct {
	Action    string   `json:"action"`  // subscribe, unsubscribe, ping, auth
	Symbols   []string `json:"symbols"` // symbols to subscribe/unsubscribe
	EventType string   `json:"event"`   // trade, ticker, depth, candle, or "all"
	Token     string   `json:"token"`   // JWT for auth action
}

// ServerResponse represents a response sent to the client.
type ServerResponse struct {
	Type    string      `json:"type"`
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Handler holds the WebSocket hub and handles upgrade requests.
type Handler struct {
	hub     *Hub
	authSvc *auth.Service
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub, authSvc *auth.Service) *Handler {
	return &Handler{hub: hub, authSvc: authSvc}
}

// ServeWS upgrades HTTP connections to WebSocket and registers the client.
func (h *Handler) HandleConnection(c *gin.Context) {
	// --- JWT Authentication (Optional for Market Data) ---
	var userID string
	tokenString := c.Query("token")
	if tokenString != "" {
		claims, err := h.authSvc.ValidateToken(tokenString)
		if err == nil {
			userID = claims.UserID
		} else {
			log.Printf("[ws] invalid token in query param: %v (connecting as guest)", err)
		}
	}
	h.handleConnectionWithUser(c, userID)
}

// HandleInternalConnection is used by internal backend bot clients.
// It only accepts loopback callers and bypasses JWT auth.
func (h *Handler) HandleInternalConnection(c *gin.Context) {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		http.Error(c.Writer, "invalid remote address", http.StatusUnauthorized)
		return
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		http.Error(c.Writer, "internal websocket endpoint is loopback-only", http.StatusUnauthorized)
		return
	}
	h.handleConnectionWithUser(c, "internal-bot")
}

func (h *Handler) handleConnectionWithUser(c *gin.Context, userID string) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:           h.hub,
		conn:          conn,
		send:          make(chan []byte, 1024),
		subscriptions: make(map[string]map[EventType]bool),
		userID:        userID,
	}

	h.hub.register <- client

	// Send welcome message
	welcome := ServerResponse{
		Type:    "connected",
		Success: true,
		Message: "WebSocket connection established",
		Data: map[string]interface{}{
			"timestamp": time.Now().UnixMilli(),
		},
	}
	if data, err := json.Marshal(welcome); err == nil {
		client.send <- data
	}

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[ws] read error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming client messages.
func (c *Client) handleMessage(data []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("invalid JSON: " + err.Error())
		return
	}

	switch msg.Action {
	case "subscribe":
		c.handleSubscribe(msg)
	case "unsubscribe":
		c.handleUnsubscribe(msg)
	case "auth":
		c.handleAuth(msg)
	case "ping":
		c.handlePing()
	case "status":
		c.handleStatus()
	default:
		c.sendError("unknown action: " + msg.Action)
	}
}

// handleAuth upgrades a guest connection to authenticated if a valid token is provided.
func (c *Client) handleAuth(msg ClientMessage) {
	if msg.Token == "" {
		c.sendError("auth: missing token")
		return
	}

	claims, err := c.hub.authSvc.ValidateToken(msg.Token)
	if err != nil {
		c.sendError("auth: invalid or expired token")
		return
	}

	// Safely update the client's identity in the hub
	c.hub.UpdateClientID(c, claims.UserID)

	c.sendSuccess("authenticated", map[string]interface{}{
		"user_id":  claims.UserID,
		"username": claims.Username,
	})
}

// handleSubscribe processes subscription requests.
func (c *Client) handleSubscribe(msg ClientMessage) {
	if len(msg.Symbols) == 0 {
		c.sendError("no symbols specified")
		return
	}

	eventType := parseEventType(msg.EventType)
	subscribedSymbols := make([]string, 0)

	for _, symbol := range msg.Symbols {
		if symbol == "" {
			continue
		}

		if msg.EventType == "all" || msg.EventType == "" {
			c.SubscribeAll(symbol)
		} else if eventType != "" {
			c.Subscribe(symbol, eventType)
		}
		subscribedSymbols = append(subscribedSymbols, symbol)
	}

	c.sendSuccess("subscribed", map[string]interface{}{
		"symbols": subscribedSymbols,
		"event":   msg.EventType,
	})
}

// handleUnsubscribe processes unsubscription requests.
func (c *Client) handleUnsubscribe(msg ClientMessage) {
	if len(msg.Symbols) == 0 {
		c.sendError("no symbols specified")
		return
	}

	eventType := parseEventType(msg.EventType)
	unsubscribedSymbols := make([]string, 0)

	for _, symbol := range msg.Symbols {
		if symbol == "" {
			continue
		}

		if msg.EventType == "all" || msg.EventType == "" {
			c.UnsubscribeAll(symbol)
		} else if eventType != "" {
			c.Unsubscribe(symbol, eventType)
		}
		unsubscribedSymbols = append(unsubscribedSymbols, symbol)
	}

	c.sendSuccess("unsubscribed", map[string]interface{}{
		"symbols": unsubscribedSymbols,
		"event":   msg.EventType,
	})
}

// handlePing responds to ping requests.
func (c *Client) handlePing() {
	response := ServerResponse{
		Type:    "pong",
		Success: true,
		Data: map[string]interface{}{
			"timestamp": time.Now().UnixMilli(),
		},
	}

	if data, err := json.Marshal(response); err == nil {
		select {
		case c.send <- data:
		default:
			log.Printf("[ws] client buffer full, dropping pong")
		}
	}
}

// handleStatus returns current subscription status.
func (c *Client) handleStatus() {
	subs := c.GetSubscriptions()

	response := ServerResponse{
		Type:    "status",
		Success: true,
		Data: map[string]interface{}{
			"subscriptions": subs,
			"timestamp":     time.Now().UnixMilli(),
		},
	}

	if data, err := json.Marshal(response); err == nil {
		select {
		case c.send <- data:
		default:
			log.Printf("[ws] client buffer full, dropping status")
		}
	}
}

// sendError sends an error response to the client.
func (c *Client) sendError(message string) {
	response := ServerResponse{
		Type:    "error",
		Success: false,
		Message: message,
	}

	if data, err := json.Marshal(response); err == nil {
		select {
		case c.send <- data:
		default:
			log.Printf("[ws] client buffer full, dropping error")
		}
	}
}

// sendSuccess sends a success response to the client.
func (c *Client) sendSuccess(message string, data interface{}) {
	response := ServerResponse{
		Type:    "success",
		Success: true,
		Message: message,
		Data:    data,
	}

	if respData, err := json.Marshal(response); err == nil {
		select {
		case c.send <- respData:
		default:
			log.Printf("[ws] client buffer full, dropping success")
		}
	}
}

// parseEventType converts string event type to EventType.
func parseEventType(s string) EventType {
	switch s {
	case "trade":
		return EventTrade
	case "ticker":
		return EventTicker
	case "depth", "orderbook":
		return EventDepth
	case "candle":
		return EventCandle
	default:
		return ""
	}
}

// GetHub returns the handler's hub instance.
func (h *Handler) GetHub() *Hub {
	return h.hub
}
