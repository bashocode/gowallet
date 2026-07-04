package handler

import (
	"net/http"

	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/service"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/gin-gonic/gin"
)

type LedgerHandler struct {
	svc service.LedgerService
}

func NewLedgerHandler(svc service.LedgerService) *LedgerHandler {
	return &LedgerHandler{svc: svc}
}

// GetMutations godoc
// @Summary		Get Mutation History
// @Description	Get a list of all mutations (credit/debit ledger entries) for the authenticated user
// @Tags		Ledger
// @Accept		json
// @Produce		json
// @Success		200 {object} map[string]interface{} "Returns success: true and data: []model.LedgerEntry"
// @Failure		401 {object} customErr.AppError
// @Failure		500 {object} customErr.AppError
// @Router		/ledger/mutations [get]
// @Security	BearerAuth
func (h *LedgerHandler) GetMutations(c *gin.Context) {
	userID, exist := c.Get("user_id")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	entries, err := h.svc.GetMutationHistory(c.Request.Context(), userIDStr)
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusNotFound, "MUTATIONS_NOT_FOUND", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entries,
	})
}

// Reconcile godoc
// @Summary		Reconcile Wallet Balance
// @Description	Verify if the current wallet balance matches the sum of ledger mutations
// @Tags		Ledger
// @Accept		json
// @Produce		json
// @Success		200 {object} map[string]interface{} "Returns success: true, is_consistent: bool, wallet_balance: decimal.Decimal, calculated_balance: decimal.Decimal"
// @Failure		401 {object} customErr.AppError
// @Failure		500 {object} customErr.AppError
// @Router		/ledger/reconcile [get]
// @Security	BearerAuth
func (h *LedgerHandler) Reconcile(c *gin.Context) {
	userID, exist := c.Get("user_id")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	isConsistent, walletBalance, calculatedBalance, err := h.svc.ReconcileWalletBalance(c.Request.Context(), userIDStr)
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusInternalServerError, "RECONCILE_FAILED", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"is_consistent":      isConsistent,
		"wallet_balance":     walletBalance.String(),
		"calculated_balance": calculatedBalance.String(),
	})
}
