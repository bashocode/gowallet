package utils

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestRetryWithBackoffSuccess(t *testing.T) {
	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	ctx := context.Background()
	err := RetryWithBackoffCustom(ctx, 5, 10*time.Millisecond, fn)

	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestRetryWithBackoffMaxRetriesExceeded(t *testing.T) {
	callCount := 0
	expectedErr := errors.New("permanent failure")
	fn := func() error {
		callCount++
		return expectedErr
	}

	ctx := context.Background()
	err := RetryWithBackoffCustom(ctx, 3, 10*time.Millisecond, fn)

	if err == nil {
		t.Error("Expected error after max retries exceeded")
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestRetryWithBackoffContextCancellation(t *testing.T) {
	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("failure")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	err := RetryWithBackoffCustom(ctx, 10, 50*time.Millisecond, fn)

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}

	if callCount >= 10 {
		t.Errorf("Expected fewer calls due to context cancellation, got %d", callCount)
	}
}

func TestRetryWithBackoffUnretryableErrors(t *testing.T) {
	t.Run("sql.ErrNoRows", func(t *testing.T) {
		callCount := 0
		fn := func() error {
			callCount++
			return sql.ErrNoRows
		}

		ctx := context.Background()
		err := RetryWithBackoffCustom(ctx, 5, 10*time.Millisecond, fn)

		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows, got %v", err)
		}
		if callCount != 1 {
			t.Errorf("Expected exactly 1 call for sql.ErrNoRows, got %d", callCount)
		}
	})

	t.Run("MarkUnretryable", func(t *testing.T) {
		callCount := 0
		customErr := errors.New("custom error")
		fn := func() error {
			callCount++
			return MarkUnretryable(customErr)
		}

		ctx := context.Background()
		err := RetryWithBackoffCustom(ctx, 5, 10*time.Millisecond, fn)

		if !errors.Is(err, customErr) {
			t.Errorf("Expected unwrapped custom error, got %v", err)
		}
		if callCount != 1 {
			t.Errorf("Expected exactly 1 call for MarkUnretryable error, got %d", callCount)
		}
	})
}

