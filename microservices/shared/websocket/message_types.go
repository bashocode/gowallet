package websocket

// Message type constants for WebSocket notifications
const (
	// MessageTypeTransferReceived is sent when a user receives a transfer
	MessageTypeTransferReceived = "transfer_received"

	// MessageTypeTransferSent is sent when a user initiates a transfer
	MessageTypeTransferSent = "transfer_sent"

	// MessageTypeTopupSuccess is sent when a payment/top-up is successful
	MessageTypeTopupSuccess = "topup_success"

	// MessageTypePaymentReceived is sent when a payment is received
	MessageTypePaymentReceived = "payment_received"

	// MessageTypeTransferFailed is sent when a transfer fails
	MessageTypeTransferFailed = "transfer_failed"

	// MessageTypePing is sent by server to keep connection alive
	MessageTypePing = "ping"

	// MessageTypePong is sent by client in response to ping
	MessageTypePong = "pong"
)
