package handler

import (
	"net/http"

	"github.com/bashocode/gowallet/monolith/internal/ledger/service"
	"github.com/gin-gonic/gin"
)

type LedgerHandler struct {
	svc service.LedgerService
}

func NewLedgerHandler(svc service.LedgerService) *LedgerHandler {
	return &LedgerHandler{svc: svc}
}

func (h *LedgerHandler) GetMutations(c *gin.Context) {
	userID, _ := c.Get("user_id")

	entries, err := h.svc.GetMutationHistory(c.Request.Context(), userID.(string))
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entries,
	})
}

func (h *LedgerHandler) Reconcile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	isConsistent, walletBalance, calculatedBalance, err := h.svc.ReconcileWalletBalance(c.Request.Context(), userID.(string))
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"is_consistent":      isConsistent,
		"wallet_balance":     walletBalance,
		"calculated_balance": calculatedBalance,
	})
}
