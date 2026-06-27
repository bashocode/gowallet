package handler

import (
	"net/http"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/gin-gonic/gin"
)

// AdminGetUsers godoc
// @Summary		Admin Get All Users
// @Description	Get all users from database with pagination (Admin only)
// @Tags		Admin
// @Accept		json
// @Produce		json
// @Param		page query int false "page number (default: 1)"
// @Param		limit query int false "page limit (default: 10)"
// @Param		sort query string false "sort column (default: created_at)"
// @Param		order query string false "sort order (default: desc)"
// @Success		200 {object} model.PaginatedResponse
// @Failure		400 {object} customErr.AppError
// @Failure		500 {object} customErr.AppError
// @Router		/admin/users [get]
func (h *UserHandler) AdminGetUsers(c *gin.Context) {
	var params model.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_INPUT", err.Error()))
		return
	}

	users, meta, err := h.svc.GetAllUsers(c.Request.Context(), params)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, model.PaginatedResponse{
		Success: true,
		Data:    users,
		Meta:    *meta,
	})
}
