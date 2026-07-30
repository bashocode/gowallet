package logger

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/utils"
)

var (
	Log *slog.Logger
	mu  sync.Mutex
)

const CorrelationIDKey = "correlation_id"

type safeWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *safeWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

func init() {
	setupLogger("unknown-service", "")
}

func InitLogger(opts ...LoggerOption) {
	config := &loggerConfig{
		serviceName:  "unknown-service",
		logstashAddr: "",
	}

	for _, opt := range opts {
		opt(config)
	}

	setupLogger(config.serviceName, config.logstashAddr)
}

func setupLogger(serviceName, logstashAddr string) {
	mu.Lock()
	defer mu.Unlock()

	var writer io.Writer = os.Stdout

	if logstashAddr != "" {
		conn, err := net.DialTimeout("tcp", logstashAddr, 2*time.Second)
		if err == nil {
			writer = &safeWriter{w: io.MultiWriter(os.Stdout, conn)}
		} else {
			println("Warning: Could not connect to Logstash TCP, logging to stdout only: " + err.Error())
			writer = &safeWriter{w: os.Stdout}
		}
	} else {
		writer = &safeWriter{w: os.Stdout}
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	Log = slog.New(handler).With("service", serviceName)
	slog.SetDefault(Log)
}

type loggerConfig struct {
	serviceName  string
	logstashAddr string
}

type LoggerOption func(*loggerConfig)

func WithServiceName(name string) LoggerOption {
	return func(c *loggerConfig) {
		c.serviceName = name
	}
}

func WithLogstashAddr(addr string) LoggerOption {
	return func(c *loggerConfig) {
		c.logstashAddr = addr
	}
}

// helper for log with context that automatically includes correlation id
func getLogArgs(ctx context.Context, args []any) []any {
	if ctx != nil {
		if cid, ok := utils.SafeString(ctx.Value(CorrelationIDKey)); ok {
			return append(args, slog.String("correlation_id", cid))
		}
	}
	return args
}

func Info(ctx context.Context, msg string, args ...any) {
	Log.InfoContext(ctx, msg, getLogArgs(ctx, args)...)
}

func Error(ctx context.Context, msg string, args ...any) {
	Log.ErrorContext(ctx, msg, getLogArgs(ctx, args)...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	Log.WarnContext(ctx, msg, getLogArgs(ctx, args)...)
}

func Fatal(ctx context.Context, msg string, args ...any) {
	Log.ErrorContext(ctx, msg, getLogArgs(ctx, args)...)
	os.Exit(1)
}
