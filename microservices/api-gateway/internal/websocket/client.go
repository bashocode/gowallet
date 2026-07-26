package websocket

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = 30 * time.Second

	// Maximum message size allowed from peer (clients only send acks)
	maxMessageSize = 512
)

// Client represents a single WebSocket connection for one user
type Client struct {
	// UserID is the authenticated user's ID from JWT
	UserID string

	// Conn is the WebSocket connection
	Conn *websocket.Conn

	// Send is a buffered channel for outbound messages
	Send chan []byte

	// hub is a reference to the Hub for unregistration
	hub *Hub
}

// readPump reads messages from the WebSocket connection
// It runs in a goroutine and handles:
// - Reading client messages (acknowledgements, pongs)
// - Detecting disconnected clients via read deadlines
// - Unregistering the client when connection closes
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.Conn.Close()
	}()

	// Set read limits and deadlines
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	// Configure pong handler to reset read deadline
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Read loop - mostly just keeps connection alive
	// Clients may send acknowledgements here in the future
	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			// Connection closed or error - exit loop
			break
		}
		// Currently we don't process client messages
		// In the future, this is where we'd handle acks: "message_received"
	}
}

// writePump writes messages from the Send channel to the WebSocket connection
// It runs in a goroutine and handles:
// - Writing messages from the Send channel
// - Sending periodic ping frames to keep connection alive
// - Gracefully closing the connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				// The hub closed the channel - close connection
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write the message
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			// Send periodic ping to keep connection alive
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
