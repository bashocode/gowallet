package websocket

import (
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// clients maps user IDs to their WebSocket connections
	// Only one connection per user (last connection wins)
	clients map[string]*Client

	// register is a channel for registering new clients
	register chan *Client

	// unregister is a channel for unregistering clients
	unregister chan *Client

	// mu protects the clients map for concurrent access
	mu sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
// This should be called in a goroutine: go hub.Run()
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()

			// If user already has a connection, close the old one
			// This handles the case where a user opens multiple tabs
			if existing, ok := h.clients[client.UserID]; ok {
				close(existing.Send)
				existing.Conn.Close()
			}

			// Register the new connection
			h.clients[client.UserID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()

			// Only unregister if this is the current connection for the user
			// (could have been replaced by a newer connection)
			if existing, ok := h.clients[client.UserID]; ok {
				if existing == client {
					delete(h.clients, client.UserID)
					close(client.Send)
				}
			}

			h.mu.Unlock()
		}
	}
}

// SendToUser sends a message to a specific user's WebSocket connection
// If the user is not connected, the message is silently dropped
// (they will see the notification when they next fetch from the API)
func (h *Hub) SendToUser(userID string, message []byte) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		// User is not connected - message is dropped
		// This is expected behavior (user might be offline)
		return
	}

	// Try to send the message
	// Use non-blocking send to avoid deadlock if client is slow
	select {
	case client.Send <- message:
		// Message sent successfully
	default:
		// Send buffer is full - client is too slow or disconnected
		// Drop the message to avoid blocking the hub
		// The client will be unregistered soon by readPump
	}
}

// GetConnectionCount returns the number of active WebSocket connections
// Useful for monitoring and debugging
func (h *Hub) GetConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// IsUserConnected checks if a specific user has an active connection
func (h *Hub) IsUserConnected(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}
