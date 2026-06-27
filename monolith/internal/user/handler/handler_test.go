package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	logger.InitLogger()
}

// MockUserService is a mock implementation of the UserService interface
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) Register(ctx context.Context, req model.CreateUserRequest) (*model.User, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserService) GetProfile(ctx context.Context, id string) (*model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserService) UpdateProfile(ctx context.Context, id string, req model.UpdateUserRequest) (*model.User, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.LoginResponse), args.Error(1)
}

func (m *MockUserService) UpdateAvatar(ctx context.Context, id string, path string) error {
	args := m.Called(ctx, id, path)
	return args.Error(0)
}

func (m *MockUserService) DeleteAccount(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserService) Logout(ctx context.Context, tokenString string) error {
	args := m.Called(ctx, tokenString)
	return args.Error(0)
}

func (m *MockUserService) VerifyEmail(ctx context.Context, userID string, code string) error {
	args := m.Called(ctx, userID, code)
	return args.Error(0)
}

func (m *MockUserService) GenerateAndSendOTP(ctx context.Context, userID string, email string, otpType string) error {
	args := m.Called(ctx, userID, email, otpType)
	return args.Error(0)
}

func (m *MockUserService) RequestPasswordReset(ctx context.Context, email string) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *MockUserService) VerifyPasswordReset(ctx context.Context, email string, code string) (string, error) {
	args := m.Called(ctx, email, code)
	return args.String(0), args.Error(1)
}

func (m *MockUserService) ResetPassword(ctx context.Context, email string, newPassword string) error {
	args := m.Called(ctx, email, newPassword)
	return args.Error(0)
}

func (m *MockUserService) GetGoogleLoginURL() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockUserService) HandleGoogleCallback(ctx context.Context, code string) (*model.LoginResponse, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.LoginResponse), args.Error(1)
}

func (m *MockUserService) RefreshToken(ctx context.Context, oldTokenString string) (*model.LoginResponse, error) {
	args := m.Called(ctx, oldTokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.LoginResponse), args.Error(1)
}

func (m *MockUserService) GetAllUsers(ctx context.Context) ([]*model.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.User), args.Error(1)
}

// ErrorHandler is copied from middleware for unit tests simplicity in this package
func testErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if appErr, ok := err.(*customErr.AppError); ok {
				c.JSON(appErr.StatusCode, gin.H{"success": false, "error": appErr})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": customErr.ErrInternalServer})
		}
	}
}

func TestRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/register", h.Register)

		reqPayload := model.CreateUserRequest{
			FullName: "John Doe",
			Email:    "john@example.com",
			Password: "password123",
		}

		expectedUser := &model.User{
			ID:       "user-uuid-123",
			FullName: "John Doe",
			Email:    "john@example.com",
		}

		mockSvc.On("Register", mock.Anything, reqPayload).Return(expectedUser, nil)

		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var respUser model.User
		json.Unmarshal(w.Body.Bytes(), &respUser)
		assert.Equal(t, expectedUser.ID, respUser.ID)
		assert.Equal(t, expectedUser.FullName, respUser.FullName)
		mockSvc.AssertExpectations(t)
	})

	t.Run("invalid payload", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/register", h.Register)

		req, _ := http.NewRequest(http.MethodPost, "/register", bytes.NewBufferString("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("service failure", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/register", h.Register)

		reqPayload := model.CreateUserRequest{
			FullName: "John Doe",
			Email:    "john@example.com",
			Password: "password123",
		}

		mockSvc.On("Register", mock.Anything, reqPayload).Return(nil, customErr.NewAppError(http.StatusConflict, "EMAIL_EXISTS", "email exists"))

		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		mockSvc.AssertExpectations(t)
	})
}

func TestLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/login", h.Login)

		reqPayload := model.LoginRequest{
			Email:    "john@example.com",
			Password: "password123",
		}

		expectedLoginResponse := &model.LoginResponse{
			AccessToken:  "access-token-123",
			RefreshToken: "refresh-token-123",
		}

		mockSvc.On("Login", mock.Anything, reqPayload).Return(expectedLoginResponse, nil)

		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))
		data := resp["data"].(map[string]interface{})
		assert.Equal(t, expectedLoginResponse.AccessToken, data["access_token"])
		mockSvc.AssertExpectations(t)
	})

	t.Run("service failure", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/login", h.Login)

		reqPayload := model.LoginRequest{
			Email:    "john@example.com",
			Password: "wrongpassword",
		}

		mockSvc.On("Login", mock.Anything, reqPayload).Return(nil, customErr.NewAppError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid credentials"))

		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mockSvc.AssertExpectations(t)
	})
}

func TestGetProfileMe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.GET("/me", func(c *gin.Context) {
			c.Set("user_id", "user-uuid-123")
		}, h.GetProfileMe)

		expectedUser := &model.User{
			ID:       "user-uuid-123",
			FullName: "John Doe",
			Email:    "john@example.com",
		}

		mockSvc.On("GetProfile", mock.Anything, "user-uuid-123").Return(expectedUser, nil)

		req, _ := http.NewRequest(http.MethodGet, "/me", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))
		data := resp["data"].(map[string]interface{})
		assert.Equal(t, expectedUser.ID, data["id"])
		mockSvc.AssertExpectations(t)
	})
}

func TestLogout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/logout", func(c *gin.Context) {
			c.Set("token_string", "token123")
		}, h.Logout)

		mockSvc.On("Logout", mock.Anything, "token123").Return(nil)

		req, _ := http.NewRequest(http.MethodPost, "/logout", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))
		assert.Contains(t, resp["message"], "Logout successful")
		mockSvc.AssertExpectations(t)
	})

	t.Run("service failure", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/logout", func(c *gin.Context) {
			c.Set("token_string", "token123")
		}, h.Logout)

		mockSvc.On("Logout", mock.Anything, "token123").Return(errors.New("service failure"))

		req, _ := http.NewRequest(http.MethodPost, "/logout", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockSvc.AssertExpectations(t)
	})
}

func TestForgotPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/forgot-password", h.ForgotPassword)

		reqPayload := PasswordResetRequest{
			Email: "test@example.com",
		}
		mockSvc.On("RequestPasswordReset", mock.Anything, "test@example.com").Return(nil)

		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest(http.MethodPost, "/forgot-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))
		assert.Contains(t, resp["message"], "If the email is registered")
		mockSvc.AssertExpectations(t)
	})

	t.Run("invalid payload", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/forgot-password", h.ForgotPassword)

		reqPayload := PasswordResetRequest{
			Email: "invalid-email",
		}

		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest(http.MethodPost, "/forgot-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockSvc.AssertExpectations(t)
	})
}

func TestVerifyPasswordReset(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/verify-password-reset", h.VerifyPasswordReset)

		reqPayload := VerifyPasswordResetRequest{
			Email:              "test@example.com",
			Code:               "123456",
			NewPassword:        "newpassword123",
			NewConfirmPassword: "newpassword123",
		}
		mockSvc.On("VerifyPasswordReset", mock.Anything, "test@example.com", "123456").Return("user-uuid", nil)
		mockSvc.On("ResetPassword", mock.Anything, "user-uuid", "newpassword123").Return(nil)

		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest(http.MethodPost, "/verify-password-reset", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))
		assert.Contains(t, resp["message"], "Password reset successfully")
		mockSvc.AssertExpectations(t)
	})

	t.Run("invalid payload - password mismatch", func(t *testing.T) {
		mockSvc := new(MockUserService)
		h := NewUserHandler(mockSvc)

		r := gin.New()
		r.Use(testErrorHandler())
		r.POST("/verify-password-reset", h.VerifyPasswordReset)

		reqPayload := VerifyPasswordResetRequest{
			Email:              "test@example.com",
			Code:               "123456",
			NewPassword:        "newpassword123",
			NewConfirmPassword: "differentpassword",
		}

		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest(http.MethodPost, "/verify-password-reset", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockSvc.AssertExpectations(t)
	})
}
