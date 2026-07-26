package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWSServer() *httptest.Server {
	var upgrader = websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
}

func TestNewHub(t *testing.T) {
	hub := NewHub()

	assert.NotNil(t, hub)
	assert.NotNil(t, hub.clients)
	assert.NotNil(t, hub.register)
	assert.NotNil(t, hub.unregister)
	assert.Equal(t, 0, len(hub.clients))
}

func TestHub_RegisterClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock WebSocket connection
	server := newTestWSServer()
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	client := &Client{
		UserID: "user123",
		Conn:   conn,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	// Register client
	hub.register <- client

	// Wait for registration to complete
	time.Sleep(50 * time.Millisecond)

	// Verify client is registered
	assert.Equal(t, 1, hub.GetConnectionCount())
	assert.True(t, hub.IsUserConnected("user123"))
}

func TestHub_UnregisterClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock WebSocket connection
	server := newTestWSServer()
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	client := &Client{
		UserID: "user123",
		Conn:   conn,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	// Register client
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Unregister client
	hub.unregister <- client
	time.Sleep(50 * time.Millisecond)

	// Verify client is unregistered
	assert.Equal(t, 0, hub.GetConnectionCount())
	assert.False(t, hub.IsUserConnected("user123"))
}

func TestHub_ReplaceExistingConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create two mock connections for the same user
	server := newTestWSServer()
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	// First connection
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn1.Close()

	client1 := &Client{
		UserID: "user123",
		Conn:   conn1,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	// Register first client
	hub.register <- client1
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 1, hub.GetConnectionCount())

	// Second connection (should replace first)
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn2.Close()

	client2 := &Client{
		UserID: "user123",
		Conn:   conn2,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	// Register second client (should replace first)
	hub.register <- client2
	time.Sleep(100 * time.Millisecond)

	// Should still have only 1 connection
	assert.Equal(t, 1, hub.GetConnectionCount())
	assert.True(t, hub.IsUserConnected("user123"))

	// Verify the active client is the second one
	hub.mu.RLock()
	activeClient := hub.clients["user123"]
	hub.mu.RUnlock()

	assert.Equal(t, client2, activeClient)
}

func TestHub_SendToUser_Connected(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		UserID: "user123",
		Conn:   nil,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	// Register client
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Send message
	message := []byte(`{"type":"test","message":"hello"}`)
	hub.SendToUser("user123", message)

	// Verify message was sent
	select {
	case msg := <-client.Send:
		assert.Equal(t, message, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Message not received within timeout")
	}
}

func TestHub_SendToUser_NotConnected(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Send message to non-existent user (should not panic)
	message := []byte(`{"type":"test","message":"hello"}`)
	assert.NotPanics(t, func() {
		hub.SendToUser("nonexistent", message)
	})
}

func TestHub_SendToUser_FullBuffer(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		UserID: "user123",
		Conn:   nil,
		Send:   make(chan []byte, 2), // Small buffer
		hub:    hub,
	}

	// Register client
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Fill the buffer
	hub.SendToUser("user123", []byte("msg1"))
	hub.SendToUser("user123", []byte("msg2"))

	// This should not block (will drop the message)
	done := make(chan bool)
	go func() {
		hub.SendToUser("user123", []byte("msg3"))
		done <- true
	}()

	select {
	case <-done:
		// Success - did not block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("SendToUser blocked when buffer was full")
	}
}

func TestHub_MultipleUsers(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Register multiple users
	users := []string{"user1", "user2", "user3"}
	clients := make([]*Client, len(users))

	for i, userID := range users {
		clients[i] = &Client{
			UserID: userID,
			Conn:   nil,
			Send:   make(chan []byte, 64),
			hub:    hub,
		}
		hub.register <- clients[i]
	}

	time.Sleep(50 * time.Millisecond)

	// Verify all users are registered
	assert.Equal(t, 3, hub.GetConnectionCount())
	for _, userID := range users {
		assert.True(t, hub.IsUserConnected(userID))
	}

	// Send message to specific user
	message := []byte("test message")
	hub.SendToUser("user2", message)

	// Verify only user2 received the message
	select {
	case msg := <-clients[1].Send:
		assert.Equal(t, message, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("user2 did not receive message")
	}

	// Verify other users did not receive the message
	select {
	case <-clients[0].Send:
		t.Fatal("user1 should not have received message")
	case <-clients[2].Send:
		t.Fatal("user3 should not have received message")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message received
	}
}

func TestHub_IsUserConnected(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// User not connected
	assert.False(t, hub.IsUserConnected("user123"))

	// Register user
	client := &Client{
		UserID: "user123",
		Conn:   nil,
		Send:   make(chan []byte, 64),
		hub:    hub,
	}

	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// User now connected
	assert.True(t, hub.IsUserConnected("user123"))

	// Unregister user
	hub.unregister <- client
	time.Sleep(50 * time.Millisecond)

	// User no longer connected
	assert.False(t, hub.IsUserConnected("user123"))
}

func TestHub_GetConnectionCount(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	assert.Equal(t, 0, hub.GetConnectionCount())

	// Add clients
	for i := 0; i < 5; i++ {
		client := &Client{
			UserID: string(rune('a' + i)),
			Conn:   nil,
			Send:   make(chan []byte, 64),
			hub:    hub,
		}
		hub.register <- client
	}

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 5, hub.GetConnectionCount())
}
