package handler

import (
	"net/http"

	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/model"
	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/service"
	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/utils"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	svc service.AuthService
}

func NewAuthHandler(svc service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req model.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	resp, err := h.svc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// The middleware should set token_string in the gin context
	tokenString, exist := c.Get("token_string")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Token context not found"))
		return
	}

	tokenStringStr, ok := utils.SafeString(tokenString)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token context"))
		return
	}

	err := h.svc.Logout(c.Request.Context(), tokenStringStr)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logout successful. Token session has been deactivated",
	})
}

// GoogleLogin redirects user to Google OAuth consent screen
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	loginURL, err := h.svc.GetGoogleLoginURL(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, loginURL)
}

// GoogleCallback handles the OAuth callback from Google
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state token is invalid"})
		return
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "authorization code is empty"})
		return
	}

	resp, err := h.svc.HandleGoogleCallback(c.Request.Context(), code, state)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
