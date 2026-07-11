package handler

import (
	"encoding/json"
	"io"
	"net/http"

	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/utils"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	transactionService "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	"github.com/gin-gonic/gin"
)

type TransferHandler struct {
	svc           transactionService.TransactionService
	webhookSecret string
}

func NewTransferHandler(svc transactionService.TransactionService, webhookSecret string) *TransferHandler {
	return &TransferHandler{svc: svc, webhookSecret: webhookSecret}
}

// CreateExternalTransfer godoc
// @Summary		Transfer to External Ewallet
// @Description	Transfer money to a user in an external ewallet (monolith). The transfer is settled when the external ewallet calls back GoWallet's webhook.
// @Tags		Transfers
// @Accept		json
// @Produce		json
// @Param		request body model.ExternalTransferRequest true "external transfer payload"
// @Success		200 {object} map[string]interface{}
// @Failure		400 {object} customErr.AppError
// @Failure		401 {object} customErr.AppError
// @Router		/transfers/external [post]
// @Security	BearerAuth
func (h *TransferHandler) CreateExternalTransfer(c *gin.Context) {
	senderUserID, exist := c.Get("user_id")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	senderUserIDStr, ok := utils.SafeString(senderUserID)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	var req model.ExternalTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	transfer, err := h.svc.CreateExternalTransfer(c.Request.Context(), senderUserIDStr, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "External transfer initiated, awaiting callback",
		"data":    transfer,
	})
}

// GetExternalTransfer godoc
// @Summary		Get External Transfer Status
// @Description	Get the current status of an external transfer by its ID. Use this to poll whether a pending transfer has been settled or failed.
// @Tags		Transfers
// @Produce		json
// @Param		id path string true "Transfer ID"
// @Success		200 {object} map[string]interface{}
// @Failure		404 {object} customErr.AppError
// @Router		/transfers/external/{id} [get]
// @Security	BearerAuth
func (h *TransferHandler) GetExternalTransfer(c *gin.Context) {
	senderUserID, exist := c.Get("user_id")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	senderUserIDStr, ok := utils.SafeString(senderUserID)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	transferID := c.Param("id")
	if transferID == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", "transfer id is required"))
		return
	}

	transfer, err := h.svc.GetExternalTransfer(c.Request.Context(), senderUserIDStr, transferID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    transfer,
	})
}

// ProcessTransferWebhook godoc
// @Summary		External Transfer Webhook
// @Description	Handle callback from external ewallet (monolith) when an outbound transfer is settled or failed.
// @Tags		Transfers
// @Accept		json
// @Produce		json
// @Param		request body model.TransferCallback true "transfer callback payload"
// @Success		200 {object} map[string]interface{}
// @Failure		400 {object} customErr.AppError
// @Router		/transfers/webhook [post]
func (h *TransferHandler) ProcessTransferWebhook(c *gin.Context) {
	// Read raw body for HMAC verification before JSON binding
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body"))
		return
	}

	// Verify HMAC-SHA256 signature from header
	signature := c.GetHeader("X-Webhook-Signature")
	if err := transactionService.VerifyWebhookSignature(rawBody, signature, h.webhookSecret); err != nil {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "INVALID_SIGNATURE", "Webhook signature verification failed"))
		return
	}

	var cb model.TransferCallback
	if err := json.Unmarshal(rawBody, &cb); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	if err := h.svc.SettleTransferTx(c.Request.Context(), cb); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transfer webhook processed successfully",
	})
}
