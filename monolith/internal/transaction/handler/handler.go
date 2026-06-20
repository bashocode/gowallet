package handler

import (
	"net/http"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/transaction/model"
	"github.com/bashocode/gowallet/monolith/internal/transaction/service"
	"github.com/gin-gonic/gin"
)

type TransactionHandler struct {
	svc service.TransactionService
}

func NewTransactionHandler(s service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: s}
}

func (h *TransactionHandler) Transfer(c *gin.Context) {
	// get senderUserID from auth middleware
	senderUserID, exist := c.Get("user_id")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	var req model.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	tx, err := h.svc.Transfer(c.Request.Context(), senderUserID.(string), req)
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

func (h *TransactionHandler) GetHistory(c *gin.Context) {
	userID, exist := c.Get("user_id")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	var params model.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	txs, meta, err := h.svc.GetHistory(c.Request.Context(), userID.(string), params)
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
