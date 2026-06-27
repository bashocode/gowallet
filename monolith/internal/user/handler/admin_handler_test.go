package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAdminGetUsers_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := new(MockUserService)
	users := []*model.User{
		{
			ID:       "user-1",
			FullName: "Admin User",
			Email:    "admin@example.com",
			Role:     "admin",
		},
		{
			ID:       "user-2",
			FullName: "Regular User",
			Email:    "user@example.com",
			Role:     "user",
		},
	}

	mockSvc.On("GetAllUsers", mock.Anything).Return(users, nil)

	h := NewUserHandler(mockSvc)

	r := gin.New()
	r.GET("/admin/users", h.AdminGetUsers)

	req, _ := http.NewRequest("GET", "/admin/users", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responseUsers []*model.User
	err := json.Unmarshal(w.Body.Bytes(), &responseUsers)
	assert.NoError(t, err)

	assert.Len(t, responseUsers, 2)
	assert.Equal(t, "user-1", responseUsers[0].ID)
	assert.Equal(t, "user-2", responseUsers[1].ID)

	mockSvc.AssertExpectations(t)
}

func TestAdminGetUsers_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := new(MockUserService)
	mockSvc.On("GetAllUsers", mock.Anything).Return(nil, customErr.ErrInternalServer)

	h := NewUserHandler(mockSvc)

	r := gin.New()
	r.Use(testErrorHandler())
	r.GET("/admin/users", h.AdminGetUsers)

	req, _ := http.NewRequest("GET", "/admin/users", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response["success"].(bool))

	mockSvc.AssertExpectations(t)
}
