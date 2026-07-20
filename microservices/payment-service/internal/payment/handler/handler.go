package handler

import (
	"html"
	"io"
	"net/http"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/model"
	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/service"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/utils"
	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	svc service.PaymentService
}

func NewPaymentHandler(svc service.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

// CreateCheckoutSession godoc
// @Summary		Create Stripe Checkout Session
// @Description	Generate a Stripe checkout link to top up balance
// @Tags		Payments
// @Accept		json
// @Produce		json
// @Param		request body model.StripeCheckoutRequest true "checkout payload"
// @Success		200 {object} model.StripeCheckoutResponse
// @Failure		400 {object} customErr.AppError
// @Failure		401 {object} customErr.AppError
// @Router		/payments/stripe/checkout [post]
// @Security	BearerAuth
func (h *PaymentHandler) CreateCheckoutSession(c *gin.Context) {
	userID, exist := c.Get("user_id")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	userIDStr, ok := utils.SafeString(userID)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	var req model.StripeCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	resp, err := h.svc.CreateCheckoutSession(c.Request.Context(), userIDStr, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Checkout session created successfully",
		"data":    resp,
	})
}

// ProcessWebhook godoc
// @Summary		Stripe Webhook Handler
// @Description	Handle Stripe callback webhooks for payment completions
// @Tags		Payments
// @Accept		json
// @Produce		json
// @Router		/payments/webhook [post]
func (h *PaymentHandler) ProcessWebhook(c *gin.Context) {
	// Read payload body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body"))
		return
	}

	sigHeader := c.GetHeader("Stripe-Signature")

	err = h.svc.ProcessWebhook(c.Request.Context(), payload, sigHeader)
	if err != nil {
		// Respond with bad request if verification or processing fails
		c.Error(customErr.NewAppError(http.StatusBadRequest, "WEBHOOK_FAILED", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webhook processed successfully",
	})
}

// SuccessCallback handles redirect from Stripe on success
func (h *PaymentHandler) SuccessCallback(c *gin.Context) {
	sessionID := html.EscapeString(c.Query("session_id"))
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Payment Successful</title>
			<style>
				body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; text-align: center; padding: 50px; background-color: #f7f9fc; }
				.card { background: white; padding: 40px; border-radius: 12px; display: inline-block; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
				h1 { color: #24b47e; }
				p { color: #4a5568; }
			</style>
		</head>
		<body>
			<div class="card">
				<h1>🎉 Top Up Success!</h1>
				<p>Thank you! Your payment session has been processed.</p>
				<p><small style="color: #a0aec0;">Session ID: `+sessionID+`</small></p>
				<p><a href="/" style="color: #635bff; text-decoration: none; font-weight: bold;">Back to GoWallet</a></p>
			</div>
		</body>
		</html>
	`))
}

// CancelCallback handles redirect from Stripe on cancel
func (h *PaymentHandler) CancelCallback(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Payment Cancelled</title>
			<style>
				body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; text-align: center; padding: 50px; background-color: #f7f9fc; }
				.card { background: white; padding: 40px; border-radius: 12px; display: inline-block; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
				h1 { color: #e53e3e; }
				p { color: #4a5568; }
			</style>
		</head>
		<body>
			<div class="card">
				<h1>❌ Top Up Cancelled</h1>
				<p>Your payment request was cancelled.</p>
				<p><a href="/" style="color: #635bff; text-decoration: none; font-weight: bold;">Back to GoWallet</a></p>
			</div>
		</body>
		</html>
	`))
}
