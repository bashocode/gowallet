package middleware

import (
	"context"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// read from request header
		corID := c.GetHeader("X-Correlation-ID")
		if corID == "" {
			// generate new uuid
			corID = uuid.New().String()
		}

		// set in request context
		ctx := context.WithValue(c.Request.Context(), logger.CorrelationIDKey, corID)
		c.Request = c.Request.WithContext(ctx)

		// set response header
		c.Header("X-Correlation-ID", corID)

		// continue
		c.Next()
	}
}
