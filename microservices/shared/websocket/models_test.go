package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebSocketMessage(t *testing.T) {
	msgType := "test_type"
	title := "Test Title"
	message := "Test Message"
	data := map[string]string{"key": "value"}

	wsMsg := NewWebSocketMessage(msgType, title, message, data)

	assert.NotNil(t, wsMsg)
	assert.Equal(t, msgType, wsMsg.Type)
	assert.Equal(t, title, wsMsg.Title)
	assert.Equal(t, message, wsMsg.Message)
	assert.Equal(t, data, wsMsg.Data)
	assert.False(t, wsMsg.Timestamp.IsZero())
}

func TestWebSocketMessage_ToJSON(t *testing.T) {
	now := time.Now()
	msg := &WebSocketMessage{
		Type:      "transfer_received",
		Title:     "Money Received",
		Message:   "You received $100",
		Data:      map[string]interface{}{"amount": 100.0},
		Timestamp: now,
	}

	jsonBytes, err := msg.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)

	// Verify JSON structure
	var decoded map[string]interface{}
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "transfer_received", decoded["type"])
	assert.Equal(t, "Money Received", decoded["title"])
	assert.Equal(t, "You received $100", decoded["message"])
	assert.NotNil(t, decoded["data"])
	assert.NotEmpty(t, decoded["timestamp"])
}

func TestWebSocketMessage_ToJSON_NoData(t *testing.T) {
	msg := &WebSocketMessage{
		Type:      "test_type",
		Title:     "Test",
		Message:   "Test message",
		Data:      nil,
		Timestamp: time.Now(),
	}

	jsonBytes, err := msg.ToJSON()
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	// Data field should be omitted when nil
	_, exists := decoded["data"]
	assert.False(t, exists)
}

func TestNewRedisNotificationPayload(t *testing.T) {
	userID := "user123"
	msg := NewWebSocketMessage("test_type", "Test", "Test message", nil)

	payload := NewRedisNotificationPayload(userID, msg)

	assert.NotNil(t, payload)
	assert.Equal(t, userID, payload.UserID)
	assert.Equal(t, msg, payload.Message)
}

func TestRedisNotificationPayload_ToJSON(t *testing.T) {
	msg := NewWebSocketMessage("transfer_sent", "Transfer Initiated", "Sending $50", map[string]interface{}{
		"amount":   50.0,
		"receiver": "user@example.com",
	})

	payload := &RedisNotificationPayload{
		UserID:  "user123",
		Message: msg,
	}

	jsonBytes, err := payload.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)

	// Verify JSON structure
	var decoded map[string]interface{}
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "user123", decoded["user_id"])
	assert.NotNil(t, decoded["message"])

	// Verify nested message structure
	messageMap := decoded["message"].(map[string]interface{})
	assert.Equal(t, "transfer_sent", messageMap["type"])
	assert.Equal(t, "Transfer Initiated", messageMap["title"])
}

func TestRedisNotificationPayload_FromJSON(t *testing.T) {
	original := &RedisNotificationPayload{
		UserID: "user123",
		Message: NewWebSocketMessage(
			"topup_success",
			"Top-up Successful",
			"Your wallet was credited",
			map[string]interface{}{"amount": 100.0},
		),
	}

	// Serialize
	jsonBytes, err := original.ToJSON()
	require.NoError(t, err)

	// Deserialize
	var decoded RedisNotificationPayload
	err = decoded.FromJSON(jsonBytes)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, original.UserID, decoded.UserID)
	assert.Equal(t, original.Message.Type, decoded.Message.Type)
	assert.Equal(t, original.Message.Title, decoded.Message.Title)
	assert.Equal(t, original.Message.Message, decoded.Message.Message)
}

func TestRedisNotificationPayload_FromJSON_Invalid(t *testing.T) {
	invalidJSON := []byte(`{"invalid": json}`)

	var payload RedisNotificationPayload
	err := payload.FromJSON(invalidJSON)
	assert.Error(t, err)
}

func TestRedisNotificationPayload_RoundTrip(t *testing.T) {
	testCases := []struct {
		name    string
		payload *RedisNotificationPayload
	}{
		{
			name: "with data",
			payload: &RedisNotificationPayload{
				UserID: "user456",
				Message: NewWebSocketMessage(
					"transfer_received",
					"Money Received",
					"You received $75",
					map[string]interface{}{
						"amount":   75.0,
						"currency": "USD",
						"sender":   "sender@example.com",
					},
				),
			},
		},
		{
			name: "without data",
			payload: &RedisNotificationPayload{
				UserID: "user789",
				Message: NewWebSocketMessage(
					"system_notification",
					"System Update",
					"System maintenance scheduled",
					nil,
				),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize
			jsonBytes, err := tc.payload.ToJSON()
			require.NoError(t, err)

			// Deserialize
			var decoded RedisNotificationPayload
			err = decoded.FromJSON(jsonBytes)
			require.NoError(t, err)

			// Verify
			assert.Equal(t, tc.payload.UserID, decoded.UserID)
			assert.Equal(t, tc.payload.Message.Type, decoded.Message.Type)
			assert.Equal(t, tc.payload.Message.Title, decoded.Message.Title)
			assert.Equal(t, tc.payload.Message.Message, decoded.Message.Message)
		})
	}
}

func TestWebSocketMessage_AllMessageTypes(t *testing.T) {
	messageTypes := []string{
		"transfer_received",
		"transfer_sent",
		"transfer_failed",
		"topup_success",
		"topup_failed",
	}

	for _, msgType := range messageTypes {
		t.Run(msgType, func(t *testing.T) {
			msg := NewWebSocketMessage(
				msgType,
				"Test Title",
				"Test Message",
				map[string]interface{}{"key": "value"},
			)

			assert.Equal(t, msgType, msg.Type)

			// Verify serialization works
			jsonBytes, err := msg.ToJSON()
			require.NoError(t, err)
			assert.NotEmpty(t, jsonBytes)
		})
	}
}
