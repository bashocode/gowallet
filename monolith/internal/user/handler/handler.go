package handler

import (
	"net/http"
	"os"
	"path/filepath"

	customError "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/bashocode/gowallet/monolith/internal/user/service"
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
// @Failure		400 {object} errors.AppError
// @Failure		409 {object} errors.AppError
// @Router		/users/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// register the error input to gin context
		c.Error(customError.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
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
// @Failure		404 {object} errors.AppError
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
// @Failure		400 {object} errors.AppError
// @Failure		404 {object} errors.AppError
// @Router		/users/{id} [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customError.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
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
// @Failure		400 {object} errors.AppError
// @Failure		401 {object} errors.AppError
// @Router		/users/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customError.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
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
// @Failure		401 {object} errors.AppError
// @Router		/users/me [get]
// @Security	BearerAuth
func (h *UserHandler) GetProfileMe(c *gin.Context) {
	userID, _ := c.Get("user_id")

	user, err := h.svc.GetProfile(c.Request.Context(), userID.(string))
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
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
// @Failure		400 {object} errors.AppError
// @Router		/users/avatar [post]
// @Security	BearerAuth
func (h *UserHandler) UploadAvatar(c *gin.Context) {
	userID, _ := c.Get("user_id")

	// get the file from request multipart
	file, err := c.FormFile("avatar")
	if err != nil {
		c.Error(customError.NewAppError(http.StatusBadRequest, "INVALID_FILE", "Please upload an avatar."))
		return
	}

	// validate file format
	if file.Size > 2*1024*1024 {
		c.Error(customError.NewAppError(http.StatusBadRequest, "INVALID_FILE", "File size must be less than 2MB."))
		return
	}

	// validate file content type
	ext := filepath.Ext(file.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.Error(customError.NewAppError(http.StatusBadRequest, "INVALID_FILE", "Invalid file format. Please upload a JPG, JPEG, or PNG image."))
		return
	}

	// folder for saving
	uploadDir := "./uploads"
	_ = os.MkdirAll(uploadDir, os.ModePerm)

	// rename file based on user id
	filename := userID.(string) + ext
	dst := filepath.Join(uploadDir, filename)

	// save the file
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.Error(customError.ErrInternalServer)
		return
	}

	// update user's avatar
	avatarURL := "/uploads/" + filename
	if err := h.svc.UpdateAvatar(c.Request.Context(), userID.(string), avatarURL); err != nil {
		c.Error(customError.ErrInternalServer)
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
// @Failure		404 {object} errors.AppError
// @Router		/users/me [delete]
// @Security	BearerAuth
func (h *UserHandler) DeleteAccount(c *gin.Context) {
	id, _ := c.Get("user_id")
	if err := h.svc.DeleteAccount(c.Request.Context(), id.(string)); err != nil {
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
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /users/logout [post]
// @Security BearerAuth
func (h *UserHandler) Logout(c *gin.Context) {
	tokenString, _ := c.Get("token_string")

	err := h.svc.Logout(c.Request.Context(), tokenString.(string))
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
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /users/verify-email [post]
// @Security BearerAuth
func (h *UserHandler) VerifyEmail(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(customError.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	err := h.svc.VerifyEmail(c.Request.Context(), userID.(string), req.Code)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Email verified successfully",
	})
}
