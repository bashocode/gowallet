package middleware

import (
	"fmt"
	"net/http"
	"time"

	customErr "github.com/bashocode/gowallet/microservices/shared/errors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RateLimiter(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		// key format: rate_limit:ip_address:minute_timestamp
		currentTime := time.Now().Unix() / int64(window.Seconds())
		key := fmt.Sprintf("rate_limit:%s:%d", ip, currentTime)

		ctx := c.Request.Context()

		// use multi/exec to atomically increment counter & set TTL
		pipe := rdb.Pipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, window*2) // store longer for safety margin

		_, err := pipe.Exec(ctx)
		if err != nil {
			c.Error(customErr.ErrInternalServer)
			c.Abort()
			return
		}

		count := incr.Val()
		if count > int64(limit) {
			c.Error(customErr.NewAppError(
				http.StatusTooManyRequests,
				"RATE_LIMIT_EXCEEDED",
				"Too many request. Please try again later.",
			))
			c.Abort()
			return
		}

		c.Next()
	}
}
