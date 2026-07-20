package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/bashocode/gowallet/microservices/shared/storage"
	"github.com/bashocode/gowallet/microservices/shared/utils"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/model"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/service"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	svc     service.UserService
	storage storage.ObjectStorage
}

func NewUserHandler(svc service.UserService, storage storage.ObjectStorage) *UserHandler {
	return &UserHandler{
		svc:     svc,
		storage: storage,
	}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	user, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	id := c.Param("id")
	user, err := h.svc.GetProfile(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	user, err := h.svc.UpdateProfile(c.Request.Context(), id, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) GetProfileMe(c *gin.Context) {
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

	user, err := h.svc.GetProfile(c.Request.Context(), userIDStr)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

func (h *UserHandler) UploadAvatar(c *gin.Context) {
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

	fileHeader, err := c.FormFile("avatar")
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE", "Please upload an avatar."))
		return
	}

	if fileHeader.Size > 2*1024*1024 {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE", "File size must be less than 2MB."))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE", "Failed to read file"))
		return
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType != "image/png" && contentType != "image/jpeg" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE", "Only PNG and JPEG formats are allowed"))
		return
	}

	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		if contentType == "image/png" {
			ext = ".png"
		} else {
			ext = ".jpg"
		}
	}

	objectName := fmt.Sprintf("avatar-%s-%d%s", userIDStr, time.Now().Unix(), ext)
	avatarURL, err := h.storage.UploadStream(c.Request.Context(), "avatars", objectName, file, fileHeader.Size, contentType)
	if err != nil {
		c.Error(customErr.ErrInternalServer)
		return
	}

	if err := h.svc.UpdateAvatar(c.Request.Context(), userIDStr, avatarURL); err != nil {
		c.Error(customErr.ErrInternalServer)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Avatar updated successfully",
		"avatar_url": avatarURL,
	})
}

func (h *UserHandler) DeleteAccount(c *gin.Context) {
	id, exist := c.Get("user_id")
	if !exist {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "User context not found"))
		return
	}

	idStr, ok := utils.SafeString(id)
	if !ok {
		c.Error(customErr.NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid user context"))
		return
	}

	if err := h.svc.DeleteAccount(c.Request.Context(), idStr); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusNoContent, gin.H{
		"success": true,
		"message": "Account deleted successfully",
	})
}

type VerifyOTPRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

func (h *UserHandler) VerifyEmail(c *gin.Context) {
	userIDStr := c.Query("user_id")
	code := c.Query("code")

	if userIDStr == "" || code == "" {
		c.Error(customErr.NewAppError(
			http.StatusBadRequest,
			"INVALID_INPUT",
			"user_id and code query parameters are required",
		))
		return
	}

	err := h.svc.VerifyEmail(c.Request.Context(), userIDStr, code)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Email verified successfully",
	})
}

type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *UserHandler) ForgotPassword(c *gin.Context) {
	var req PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	_ = h.svc.RequestPasswordReset(c.Request.Context(), req.Email)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "If the email is registered, you will receive an OTP code",
	})
}

type VerifyPasswordResetRequest struct {
	Email              string `json:"email" binding:"required,email"`
	Code               string `json:"code" binding:"required,len=6"`
	NewPassword        string `json:"new_password" binding:"required,min=6"`
	NewConfirmPassword string `json:"new_confirm_password" binding:"required,eqfield=NewPassword"`
}

func (h *UserHandler) VerifyPasswordReset(c *gin.Context) {
	var req VerifyPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if req.NewPassword != req.NewConfirmPassword {
			c.Error(customErr.NewAppError(http.StatusBadRequest, "PASSWORD_MISMATCH", "new password and confirm password do not match."))
			return
		}

		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	userID, err := h.svc.VerifyPasswordReset(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		c.Error(err)
		return
	}

	if err := h.svc.ResetPassword(c.Request.Context(), userID, req.NewPassword); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password reset successfully",
	})
}
