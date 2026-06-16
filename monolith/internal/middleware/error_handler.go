package middleware

import (
	"net/http"

	customError "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// next handler
		c.Next()

		// check if any error occurred
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// check if the error is one of our custom AppError
			if appErr, ok := err.(*customError.AppError); ok {
				logger.Warn(c.Request.Context(), "Client error occured",
					"code", appErr.Code,
					"message", appErr.Message,
					"status", appErr.StatusCode,
				)
				c.JSON(appErr.StatusCode, gin.H{
					"success": false,
					"error":   appErr,
				})
				return
			}

			// if error is not cover from our custom AppError
			logger.Error(c.Request.Context(), "Unhandled error occured", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   customError.ErrInternalServer,
			})
		}
	}
}
