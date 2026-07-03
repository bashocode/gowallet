package handler

import (
	"net/http"

	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/repository"
	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	repo repository.WalletRepository
}

func NewWalletHandler(repo repository.WalletRepository) *WalletHandler {
	return &WalletHandler{repo: repo}
}

func (h *WalletHandler) GetBalance(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	w, err := h.repo.GetByUserID(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       w.ID,
		"user_id":  w.UserID,
		"balance":  w.Balance,
		"currency": w.Currency,
		"status":   w.Status,
		"version":  w.Version,
	})
}
