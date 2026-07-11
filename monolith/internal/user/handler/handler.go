package handler

import (
	"net/http"
	"os"
	"path/filepath"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/bashocode/gowallet/monolith/internal/user/service"
	"github.com/bashocode/gowallet/monolith/internal/utils"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	svc service.UserService
}

func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Register godoc
// @Summary		Register New User
// @Description	Create new User with default wallet
// @Tags		Users
// @Accept		json
// @Produce		json
// @Param		request	body model.CreateUserRequest true "register user payload"
// @Success		201 {object} model.User
// @Failure		400 {object} customErr.AppError
// @Failure		409 {object} customErr.AppError
// @Router		/users/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// register the error input to gin context
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	user, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		// register the error to middleware
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, user)
}

// GetProfile godoc
// @Summary		Get User Profile
// @Description	Get user profile by user ID
// @Tags		Users
// @Accept		json
// @Produce		json
// @Param		id path string true "user id (uuid)"
// @Success		200 {object} model.User
// @Failure		404 {object} customErr.AppError
// @Router		/users/{id} [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
	id := c.Param("id")
	user, err := h.svc.GetProfile(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateProfile godoc
// @Summary		Update User Profile
// @Description	Update the authenticated user profile details
// @Tags		Users
// @Accept		json
// @Produce		json
// @Param		id path string true "user id"
// @Param		request body model.UpdateUserRequest true "update profile payload"
// @Success		200 {object} model.User
// @Failure		400 {object} customErr.AppError
// @Failure		404 {object} customErr.AppError
// @Router		/users/{id} [put]
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

// Login godoc
// @Summary		User Login
// @Description	Authenticate user with email and password, returning JWT access and refresh tokens
// @Tags		Users
// @Accept		json
// @Produce		json
// @Param		request body model.LoginRequest true "login payload"
// @Success		200 {object} map[string]interface{} "Returns a payload with success: true and data: model.LoginResponse"
// @Failure		400 {object} customErr.AppError
// @Failure		401 {object} customErr.AppError
// @Router		/users/login [post]
func (h *UserHandler) Login(c *gin.Context) {
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

// GetProfileMe godoc
// @Summary		Get My Profile
// @Description	Get current authenticated user profile using token
// @Tags		Users
// @Accept		json
// @Produce		json
// @Success		200 {object} map[string]interface{} "Returns success: true and data: model.User"
// @Failure		401 {object} customErr.AppError
// @Router		/users/me [get]
// @Security	BearerAuth
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

// GetUserByEmail godoc
// @Summary		Get User by Email
// @Description	Look up a user by email. Used by GoWallet microservice for external transfer receiver validation.
// @Tags		Users
// @Produce		json
// @Param		email path string true "User email"
// @Success		200 {object} map[string]interface{}
// @Failure		404 {object} customErr.AppError
// @Router		/users/email/{email} [get]
func (h *UserHandler) GetUserByEmail(c *gin.Context) {
	email := c.Param("email")
	if email == "" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "BAD_REQUEST", "email is required"))
		return
	}

	user, err := h.svc.GetUserByEmail(c.Request.Context(), email)
	if err != nil || user == nil {
		c.Error(customErr.NewAppError(http.StatusNotFound, "USER_NOT_FOUND", "User not found"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":        user.ID,
			"full_name": user.FullName,
			"email":     user.Email,
		},
	})
}

// UploadAvatar godoc
// @Summary		Upload Avatar
// @Description	Upload profile picture avatar (JPG, JPEG, PNG, max 2MB)
// @Tags		Users
// @Accept		multipart/form-data
// @Produce		json
// @Param		avatar formData file true "Avatar image file"
// @Success		200 {object} map[string]interface{} "Returns success: true, message: Success, and avatar_url: string"
// @Failure		400 {object} customErr.AppError
// @Router		/users/avatar [post]
// @Security	BearerAuth
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

	// get the file from request multipart
	file, err := c.FormFile("avatar")
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE", "Please upload an avatar."))
		return
	}

	// validate file format
	if file.Size > 2*1024*1024 {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE", "File size must be less than 2MB."))
		return
	}

	// validate file content type
	ext := filepath.Ext(file.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE", "Invalid file format. Please upload a JPG, JPEG, or PNG image."))
		return
	}

	// folder for saving
	uploadDir := "./uploads"
	_ = os.MkdirAll(uploadDir, os.ModePerm)

	// rename file based on user id
	filename := userIDStr + ext
	dst := filepath.Join(uploadDir, filename)

	// save the file
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.Error(customErr.ErrInternalServer)
		return
	}

	// update user's avatar
	avatarURL := "/uploads/" + filename
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

// DeleteAccount godoc
// @Summary		Delete User Account
// @Description	Soft delete user account
// @Tags		Users
// @Success		204 {object} map[string]interface{}
// @Failure		404 {object} customErr.AppError
// @Router		/users/me [delete]
// @Security	BearerAuth
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

// Logout godoc
// @Summary Logout
// @Description Logout user and invalidate current session
// @Tags Users
// @Produce json
// @Success 200 {object} map[string]interface{} "Returns success and message"
// @Failure 400 {object} customErr.AppError
// @Failure 401 {object} customErr.AppError
// @Failure 500 {object} customErr.AppError
// @Router /users/logout [post]
// @Security BearerAuth
func (h *UserHandler) Logout(c *gin.Context) {
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

type VerifyOTPRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// VerifyEmail godoc
// @Summary Verify Email
// @Description Verify user email using OTP code
// @Tags Users
// @Produce json
// @Param request body VerifyOTPRequest true "otp code"
// @Success 200 {object} map[string]interface{} "Returns success and message"
// @Failure 400 {object} customErr.AppError
// @Failure 401 {object} customErr.AppError
// @Failure 500 {object} customErr.AppError
// @Router /users/verify-email [post]
// @Security BearerAuth
func (h *UserHandler) VerifyEmail(c *gin.Context) {
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

	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	err := h.svc.VerifyEmail(c.Request.Context(), userIDStr, req.Code)
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

// ForgotPassword godoc
// @Summary Forgot Password
// @Description Send OTP code to user's email for password reset
// @Tags Users
// @Produce json
// @Param request body PasswordResetRequest true "email"
// @Success 200 {object} map[string]interface{} "Returns success and message"
// @Failure 400 {object} customErr.AppError
// @Failure 401 {object} customErr.AppError
// @Failure 500 {object} customErr.AppError
// @Router /users/forgot-password [post]
func (h *UserHandler) ForgotPassword(c *gin.Context) {
	var req PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	// always return success
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

// VerifyPasswordReset godoc
// @Summary Verify Password Reset
// @Description Verify user password reset using OTP code
// @Tags Users
// @Produce json
// @Param request body VerifyPasswordResetRequest true "email, code, new_password, new_confirm_password"
// @Success 200 {object} map[string]interface{} "Returns success and message"
// @Failure 400 {object} customErr.AppError
// @Failure 401 {object} customErr.AppError
// @Failure 500 {object} customErr.AppError
// @Router /users/verify-password-reset [post]
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

// GoogleLogin godoc
// @Summary Google Login
// @Description Redirect to Google OAuth login page
// @Tags Users
// @Produce json
// @Success 302 {object} map[string]interface{} "Redirect to Google OAuth page"
// @Failure 400 {object} customErr.AppError
// @Failure 500 {object} customErr.AppError
// @Router /users/google/login [get]
func (h *UserHandler) GoogleLogin(c *gin.Context) {
	loginURL, err := h.svc.GetGoogleLoginURL(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, loginURL)
}

// GoogleCallback godoc
// @Summary Google Callback
// @Description Handle callback from Google OAuth
// @Tags Users
// @Produce json
// @Param code query string true "OAuth code"
// @Param state query string true "OAuth state"
// @Success 200 {object} map[string]interface{} "Returns login response"
// @Failure 400 {object} customErr.AppError
// @Failure 500 {object} customErr.AppError
// @Router /users/google/callback [get]
// @Security BearerAuth
func (h *UserHandler) GoogleCallback(c *gin.Context) {
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

// RefreshToken godoc
// @Summary Refresh Token
// @Description Refresh token
// @Tags Users
// @Produce json
// @Param request body model.RefreshTokenRequest true "refresh token"
// @Success 200 {object} map[string]interface{} "Returns success and message"
// @Failure 400 {object} customErr.AppError
// @Failure 401 {object} customErr.AppError
// @Failure 500 {object} customErr.AppError
// @Router /users/refresh-token [post]
func (h *UserHandler) RefreshToken(c *gin.Context) {
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
