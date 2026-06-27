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

	meta := &model.PaginationMeta{
		Page:      1,
		Limit:     10,
		Total:     2,
		TotalPage: 1,
	}

	defaultParams := model.PaginationParams{
		Page:  1,
		Limit: 10,
		Sort:  "created_at",
		Order: "desc",
	}

	mockSvc.On("GetAllUsers", mock.Anything, defaultParams).Return(users, meta, nil)

	h := NewUserHandler(mockSvc)

	r := gin.New()
	r.GET("/admin/users", h.AdminGetUsers)

	req, _ := http.NewRequest("GET", "/admin/users", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response model.PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response.Success)

	var responseUsers []*model.User
	dataBytes, _ := json.Marshal(response.Data)
	err = json.Unmarshal(dataBytes, &responseUsers)
	assert.NoError(t, err)

	assert.Len(t, responseUsers, 2)
	assert.Equal(t, "user-1", responseUsers[0].ID)
	assert.Equal(t, "user-2", responseUsers[1].ID)
	assert.Equal(t, int64(2), response.Meta.Total)

	mockSvc.AssertExpectations(t)
}

func TestAdminGetUsers_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := new(MockUserService)
	defaultParams := model.PaginationParams{
		Page:  1,
		Limit: 10,
		Sort:  "created_at",
		Order: "desc",
	}
	mockSvc.On("GetAllUsers", mock.Anything, defaultParams).Return(nil, nil, customErr.ErrInternalServer)

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
