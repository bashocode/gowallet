package handler

import (
	"net/http"

	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	transactionService "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	"github.com/gin-gonic/gin"
)

type TransferHandler struct {
	svc             transactionService.TransactionService
	webhookSecret   string
	monolithBaseURL string
}

func NewTransferHandler(svc transactionService.TransactionService, webhookSecret string, monolithBaseURL string) *TransferHandler {
	return &TransferHandler{svc: svc, webhookSecret: webhookSecret, monolithBaseURL: monolithBaseURL}
}

// InquiryExternal godoc
// @Summary		Inquiry External Wallet Email
// @Description	Validate if an email is registered in monolith system before performing transactions
// @Tags		Transactions
// @Accept		json
// @Produce		json
// @Param		request body model.ExternalInquiryRequest true "inquiry request payload"
// @Success		200 {object} map[string]interface{} "Returns success: true and data: model.WalletInquiry"
// @Failure		400 {object} customErr.AppError
// @Failure		404 {object} customErr.AppError
// @Router		/transactions/inquiry/external [post]
// @Security	BearerAuth
func (h *TransferHandler) InquiryExternal(c *gin.Context) {
	var req model.ExternalInquiryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}

	inquiry, err := h.svc.ValidateExternalEmail(c.Request.Context(), req.Email, h.webhookSecret, h.monolithBaseURL)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    inquiry,
	})
}
