package handler

import (
	"net/http"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/utils"
	"github.com/bashocode/gowallet/monolith/internal/wallet/service"
	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	svc service.WalletService
}

func NewWalletHandler(s service.WalletService) *WalletHandler {
	return &WalletHandler{svc: s}
}

// GetMyWallet godoc
// @Summary		Get My Wallet
// @Description	Get current authenticated user's wallet info (balance, currency, etc.)
// @Tags		Wallets
// @Accept		json
// @Produce		json
// @Success		200 {object} map[string]interface{} "Returns success: true and data: model.Wallet"
// @Failure		401 {object} customErr.AppError
// @Failure		404 {object} customErr.AppError
// @Router		/wallets/me [get]
// @Security	BearerAuth
// func (h *WalletHandler) GetMyWallet(c *gin.Context)
func (h *WalletHandler) GetMyWallet(c *gin.Context) {
	// user_id from jwt context
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

	wallet, err := h.svc.GetWalletByUserID(c.Request.Context(), userIDStr)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    wallet,
	})
}
