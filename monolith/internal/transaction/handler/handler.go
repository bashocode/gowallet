package handler

import (
	"net/http"
	"reflect"

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
	svc service.TransactionService
}

func NewTransactionHandler(s service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: s}
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
// @Description	Service-to-service endpoint used by transaction-service to credit a monolith wallet.
// @Tags		Transactions
// @Accept		json
// @Produce		json
// @Param		request body model.ExternalTransferRequest true "external transfer payload"
// @Success		200 {object} map[string]interface{}
// @Failure		400 {object} customErr.AppError
// @Failure		401 {object} customErr.AppError
// @Router		/transfers/external [post]
// @Security	ApiKeyAuth
func (h *TransactionHandler) ReceiveExternalTransfer(c *gin.Context) {
	var req model.ExternalTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	result, err := h.svc.ReceiveExternalTransfer(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetExternalTransferStatus godoc
// @Summary		Get External Transfer Status
// @Description	Service-to-service endpoint used by transaction-service reconciliation.
// @Tags		Transactions
// @Produce		json
// @Param		id path string true "Transfer ID"
// @Success		200 {object} map[string]interface{}
// @Failure		401 {object} customErr.AppError
// @Failure		404 {object} customErr.AppError
// @Router		/transfers/external/{id}/status [get]
// @Security	ApiKeyAuth
func (h *TransactionHandler) GetExternalTransferStatus(c *gin.Context) {
	transferID := c.Param("id")
	if transferID == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", "transfer id is required"))
		return
	}

	result, err := h.svc.GetExternalTransferStatus(c.Request.Context(), transferID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
