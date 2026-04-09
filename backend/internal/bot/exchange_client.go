package bot

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// ExchangeClient wraps a websocket connection to the matching engine.
type ExchangeClient struct {
	conn     *websocket.Conn
	writeMu  sync.Mutex // gorilla/websocket doesn't allow concurrent writes
	clientID string
}

// NewExchangeClient creates a client that auto-injects the given clientID.
func NewExchangeClient(clientID string) *ExchangeClient {
	return &ExchangeClient{clientID: clientID}
}

// Connect dials the exchange websocket at the given URL.
func (c *ExchangeClient) Connect(wsURL string) error {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws connect failed: %w", err)
	}
	c.conn = conn
	return nil
}

// Send marshals msg to JSON and writes it over the websocket.
// It auto-injects the client_id field.
func (c *ExchangeClient) Send(msg OutgoingOrder) error {
	msg.ClientID = c.clientID

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, data)
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
			if err := json.Unmarshal([]byte(part), &msg); err != nil {
				continue
			}
			return msg, nil
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
