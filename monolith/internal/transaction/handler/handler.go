package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"reflect"
	"time"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/transaction/model"
	"github.com/bashocode/gowallet/monolith/internal/transaction/service"
	"github.com/bashocode/gowallet/monolith/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
			if val, ok := utils.SafeDecimal(field.Interface()); ok {
				d, _ := val.Float64()
				return d
			}
			return nil
		}, decimal.Decimal{})
	}
}

type TransactionHandler struct {
	svc           service.TransactionService
	callbackURL   string
	webhookSecret string
	httpClient   *http.Client
}

func NewTransactionHandler(s service.TransactionService, callbackURL string, webhookSecret string) *TransactionHandler {
	if callbackURL == "" {
		callbackURL = "http://localhost:8080"
	}
	return &TransactionHandler{
		svc:           s,
		callbackURL:   callbackURL,
		webhookSecret: webhookSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Transfer godoc
// @Summary		Transfer Balance
// @Description	Transfer money to another user's wallet using email
// @Tags		Transactions
// @Accept		json
// @Produce		json
// @Param		request body model.TransferRequest true "transfer payload"
// @Success		200 {object} map[string]interface{} "Returns success: true, message: Success, and data: model.Transaction"
// @Failure		400 {object} customErr.AppError
// @Failure		401 {object} customErr.AppError
// @Failure		409 {object} customErr.AppError
// @Router		/transactions/transfer [post]
// @Security	BearerAuth
func (h *TransactionHandler) Transfer(c *gin.Context) {
	// get senderUserID from auth middleware
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

	var req model.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	tx, err := h.svc.Transfer(c.Request.Context(), senderUserIDStr, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Transaction successful",
		"data":    tx,
	})
}

// GetHistory godoc
// @Summary		Get Transaction History
// @Description	Get paginated list of transactions involving the authenticated user
// @Tags		Transactions
// @Accept		json
// @Produce		json
// @Param		page query int false "page number (default: 1)"
// @Param		limit query int false "page limit (default: 10)"
// @Param		sort query string false "sort column (default: created_at)"
// @Param		order query string false "sort order (default: desc)"
// @Param		status query string false "filter by status (success/failed)"
// @Success		200 {object} model.PaginatedResponse
// @Failure		400 {object} customErr.AppError
// @Failure		401 {object} customErr.AppError
// @Router		/transactions/history [get]
// @Security	BearerAuth
func (h *TransactionHandler) GetHistory(c *gin.Context) {
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

	var params model.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	txs, meta, err := h.svc.GetHistory(c.Request.Context(), userIDStr, params)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, model.PaginatedResponse{
		Success: true,
		Data:    txs,
		Meta:    *meta,
	})
}

// TopUp godoc
// @Summary		Top Up Wallet Balance
// @Description	Top up authenticated user's own wallet balance
// @Tags		Transactions
// @Accept		json
// @Produce		json
// @Param		request body model.TopUpRequest true "topup payload"
// @Success		200 {object} map[string]interface{} "Returns success: true, message: Success, and data: model.Transaction"
// @Failure		400 {object} customErr.AppError
// @Failure		401 {object} customErr.AppError
// @Router		/transactions/topup [post]
// @Security	BearerAuth
func (h *TransactionHandler) TopUp(c *gin.Context) {
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

	var req model.TopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	tx, err := h.svc.TopUp(c.Request.Context(), userIDStr, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Top up successful",
		"data":    tx,
	})
}

// ReceiveExternalTransfer godoc
// @Summary		Receive External Transfer
// @Description	Receive a transfer from GoWallet microservice. Credits the receiver wallet and calls back GoWallet's webhook with the result.
// @Tags		Transactions
// @Accept		json
// @Produce		json
// @Param		request body model.ExternalTransferPayload true "external transfer payload"
// @Success		200 {object} map[string]interface{}
// @Failure		400 {object} customErr.AppError
// @Router		/transfers/external [post]
func (h *TransactionHandler) ReceiveExternalTransfer(c *gin.Context) {
	var req model.ExternalTransferPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	status, err := h.svc.ReceiveExternalTransfer(c.Request.Context(), req)
	if err != nil {
		// Even on failure, we call back GoWallet so it can mark the transfer as failed.
		go h.callbackGoWallet(req, "failed")
		c.Error(err)
		return
	}

	// Async callback to GoWallet microservice webhook.
	go h.callbackGoWallet(req, status)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "External transfer received, callback dispatched",
		"data": gin.H{
			"transfer_id": req.TransferID,
			"status":      status,
		},
	})
}

// callbackGoWallet POSTs the transfer result back to GoWallet's webhook with
// an HMAC-SHA256 signature so GoWallet can verify authenticity.
func (h *TransactionHandler) callbackGoWallet(req model.ExternalTransferPayload, status string) {
	payload, _ := json.Marshal(map[string]any{
		"transfer_id":     req.TransferID,
		"status":          status,
		"receiver_email":  req.ReceiverEmail,
		"amount":          req.Amount,
		"idempotency_key": req.IdempotencyKey,
	})

	// Compute HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	reqHTTP, err := http.NewRequest("POST", h.callbackURL+"/api/v1/transfers/webhook", bytes.NewBuffer(payload))
	if err != nil {
		return
	}
	reqHTTP.Header.Set("Content-Type", "application/json")
	reqHTTP.Header.Set("X-Webhook-Signature", signature)

	resp, err := h.httpClient.Do(reqHTTP)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// GetExternalTransferStatus godoc
// @Summary		Get External Transfer Status
// @Description	Query the status of an external transfer by idempotency key. Used by GoWallet's reconciliation worker.
// @Tags		Transactions
// @Produce		json
// @Param		idempotency_key path string true "Idempotency Key"
// @Success		200 {object} map[string]interface{}
// @Router		/transfers/external/{idempotency_key}/status [get]
func (h *TransactionHandler) GetExternalTransferStatus(c *gin.Context) {
	idempotencyKey := c.Param("idempotency_key")
	if idempotencyKey == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", "idempotency_key is required"))
		return
	}

	status, err := h.svc.GetExternalTransferStatus(c.Request.Context(), idempotencyKey)
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusNotFound, "NOT_FOUND", "Transfer not found"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status": status,
		},
	})
}
