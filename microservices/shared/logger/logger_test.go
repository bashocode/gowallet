package logger

import (
	"context"
	"testing"
)

func TestInitLogger(t *testing.T) {
	InitLogger()
	if Log == nil {
		t.Fatal("expected logger.Log to be initialized, got nil")
	}
}

func TestLoggerHelpers(t *testing.T) {
	InitLogger()

	ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation-id")

	// Call helpers to ensure no panic and that they process context correctly
	Info(ctx, "info message", "key", "value")
	Warn(ctx, "warn message", "key", "value")
	Error(ctx, "error message", "key", "value")

	// Test nil context
	Info(context.Background(), "nil context message")
}

func TestInitLoggerWithOptions(t *testing.T) {
	InitLogger(
		WithServiceName("test-service"),
		WithLogstashAddr("127.0.0.1:59999"), // Unreachable port to test fallback
	)

	if Log == nil {
		t.Fatal("expected logger.Log to be initialized, got nil")
	}
}
