package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/model"
	"github.com/bashocode/gowallet/microservices/shared/database"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
)

type WebhookHandler struct {
	rabbitmqURL string
	amqpConn    *amqp.Connection
	channel     *amqp.Channel
	secretKey   []byte
	mu          sync.Mutex
}

func NewWebhookHandler(rabbitmqURL string, secret string) *WebhookHandler {
	h := &WebhookHandler{
		rabbitmqURL: rabbitmqURL,
		secretKey:   []byte(secret),
	}

	// Connect on initialization to fail fast if config is wrong
	if err := h.ensureConnection(); err != nil {
		logger.Fatal(nil, "Failed to initialize RabbitMQ connection", "error", err)
	}

	return h
}

func (h *WebhookHandler) ensureConnection() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.amqpConn == nil || h.amqpConn.IsClosed() {
		logger.Log.Info("Connecting/Reconnecting to RabbitMQ in Payment Service...")
		conn, err := database.ConnectRabbitMQ(h.rabbitmqURL)
		if err != nil {
			return err
		}
		h.amqpConn = conn

		ch, err := conn.Channel()
		if err != nil {
			h.amqpConn.Close()
			h.amqpConn = nil
			return err
		}
		h.channel = ch

		// Declare main Exchange for wallet transactions
		err = ch.ExchangeDeclare(
			"wallet.events", // exchange name
			"topic",         // exchange type
			true,            // durable
			false,           // auto-deleted
			false,           // internal
			false,           // no-wait
			nil,             // arguments
		)
		if err != nil {
			h.channel.Close()
			h.channel = nil
			h.amqpConn.Close()
			h.amqpConn = nil
			return err
		}
		logger.Log.Info("Successfully connected to RabbitMQ and declared exchange in Payment Service!")
	}
	return nil
}

func (h *WebhookHandler) HandleWebhookCallback(c *gin.Context) {
	// Ensure connection before processing
	if err := h.ensureConnection(); err != nil {
		logger.Error(c.Request.Context(), "Failed to connect to RabbitMQ for webhook callback", "error", err.Error())
		c.Error(customErr.ErrInternalServer)
		return
	}

	// 1. Read raw request body (raw bytes) for HMAC calculation
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Error(customErr.ErrBadRequest)
		return
	}
	// Return body to request context so Gin can bind JSON afterwards
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 2. Get signature from HTTP header
	signatureHeader := c.GetHeader("X-Callback-Signature")
	if signatureHeader == "" {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "MISSING_SIGNATURE", "Signature header required."))
		c.Abort()
		return
	}

	// 3. Calculate local HMAC-SHA256 signature
	mac := hmac.New(sha256.New, h.secretKey)
	mac.Write(bodyBytes)
	expectedMac := mac.Sum(nil)
	expectedSignature := hex.EncodeToString(expectedMac)

	// 4. Compare signatures (Use ConstantTimeCompare to avoid timing attack)
	if !hmac.Equal([]byte(signatureHeader), []byte(expectedSignature)) {
		logger.Warn(c.Request.Context(), "Spoofed webhook request detected! Signature mismatch.")
		c.Error(customErr.NewAppError(http.StatusForbidden, "INVALID_SIGNATURE", "Request signature is invalid."))
		c.Abort()
		return
	}

	// 5. Bind JSON to model if signature is valid
	var callback model.PaymentGatewayCallback
	if err := c.ShouldBindJSON(&callback); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	// 6. If payment status is "settled" (successfully paid)
	if callback.PaymentStatus == "settled" {
		logger.Info(c.Request.Context(), "Payment verified. Publishing event...", "order_id", callback.OrderID)

		// Compose event payload
		eventPayload, _ := json.Marshal(map[string]any{
			"user_id":  callback.UserID,
			"amount":   callback.Amount,
			"order_id": callback.OrderID,
			"gateway":  callback.PaymentGateway,
		})

		// Publish "payment.completed" event to RabbitMQ Exchange
		h.mu.Lock()
		err = h.channel.PublishWithContext(
			c.Request.Context(),
			"wallet.events",     // exchange
			"payment.completed", // routing key
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        eventPayload,
				MessageId:   callback.OrderID,
			},
		)
		h.mu.Unlock()

		if err != nil {
			logger.Error(c.Request.Context(), "Failed to publish payment.completed event", "error", err.Error())

			// Close connection and channel to trigger reconnect next time
			h.mu.Lock()
			if h.channel != nil {
				_ = h.channel.Close()
				h.channel = nil
			}
			if h.amqpConn != nil {
				_ = h.amqpConn.Close()
				h.amqpConn = nil
			}
			h.mu.Unlock()

			c.Error(customErr.ErrInternalServer)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webhook processed successfully.",
	})
}
