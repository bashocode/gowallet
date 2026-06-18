package handler

import (
	"net/http"

	"github.com/bashocode/gowallet/monolith/internal/wallet/service"
	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	svc service.WalletService
}

func NewWalletHandler(s service.WalletService) *WalletHandler {
	return &WalletHandler{svc: s}
}

func (h *WalletHandler) GetMyWallet(c *gin.Context) {
	// user_id from jwt context
	userID, _ := c.Get("user_id")

	wallet, err := h.svc.GetWalletByUserID(c.Request.Context(), userID.(string))
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    wallet,
	})
}
