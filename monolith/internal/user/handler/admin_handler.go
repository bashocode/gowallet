package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminGetUsers godoc
// @Summary		Admin Get All Users
// @Description	Get all users from database (Admin only)
// @Tags		Admin
// @Accept		json
// @Produce		json
// @Success		200 {array} model.User
// @Failure		500 {object} errors.AppError
// @Router		/admin/users [get]
func (h *UserHandler) AdminGetUsers(c *gin.Context) {
	users, err := h.svc.GetAllUsers(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, users)
}
