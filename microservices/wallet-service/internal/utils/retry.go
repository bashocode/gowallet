package utils

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
)

type unretryableError struct {
	err error
}

func (e unretryableError) Error() string {
	return e.err.Error()
}

func (e unretryableError) Unwrap() error {
	return e.err
}

// MarkUnretryable wraps an error to indicate it should not be retried by RetryWithBackoff.
func MarkUnretryable(err error) error {
	if err == nil {
		return nil
	}
	return unretryableError{err: err}
}

// RetryWithBackoff executes fn with exponential backoff and jitter up to maxRetries using a default 1s base backoff.
func RetryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	return RetryWithBackoffCustom(ctx, maxRetries, 1*time.Second, fn)
}

// RetryWithBackoffCustom executes fn with exponential backoff and jitter using a custom base backoff duration.
func RetryWithBackoffCustom(ctx context.Context, maxRetries int, baseBackoff time.Duration, fn func() error) error {
	var lastErr error

	for i := range maxRetries {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn()
		if err == nil {
			return nil
		}

		var unretryable unretryableError
		if errors.Is(err, sql.ErrNoRows) || errors.As(err, &unretryable) {
			return err
		}

		lastErr = err

		if i < maxRetries-1 {
			backoff := time.Duration(float64(baseBackoff) * math.Pow(2, float64(i)))

			jitter := time.Duration(rand.Float64() * 0.5 * float64(backoff))
			sleepTime := backoff + jitter - (backoff / 4)

			logger.Warn(ctx, "Operation failed, retrying...", "attempt", i+1, "sleep_time", sleepTime, "error", err.Error())

			timer := time.NewTimer(sleepTime)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return lastErr
}

