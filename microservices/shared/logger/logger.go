package logger

import (
	"context"
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

type ContextKey string

const CorrelationIDKey ContextKey = "correlation_id"

type safeWriter struct {
	mu          sync.Mutex
	addr        string
	conn        net.Conn
	lastAttempt time.Time
	backoff     time.Duration
}

func newSafeWriter(addr string) *safeWriter {
	return &safeWriter{
		addr:    addr,
		backoff: 1 * time.Second, // initial backoff 1 second
	}
}

func (s *safeWriter) Write(p []byte) (n int, err error) {
	// 1. Always write to os.Stdout so console/docker logs are never lost
	n, err = os.Stdout.Write(p)

	// If no Logstash address configured, return
	if s.addr == "" {
		return n, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 2. Exponential Backoff Reconnection Logic
	if s.conn == nil {
		if s.backoff < 1*time.Second {
			s.backoff = 1 * time.Second
		}

		if time.Since(s.lastAttempt) >= s.backoff {
			s.lastAttempt = time.Now()
			conn, dialErr := net.DialTimeout("tcp", s.addr, 2*time.Second)
			if dialErr == nil {
				s.conn = conn
				s.backoff = 1 * time.Second // Reset backoff on successful connection
			} else {
				// Exponential backoff: double the delay up to a max of 30 seconds
				s.backoff *= 2
				if s.backoff > 30*time.Second {
					s.backoff = 30 * time.Second
				}
			}
		}
	}

	// 3. Write to Logstash if connection is active with 2s write deadline
	if s.conn != nil {
		_ = s.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, writeErr := s.conn.Write(p)
		if writeErr != nil {
			// On error, close socket and reset state so it retries with backoff next time
			_ = s.conn.Close()
			s.conn = nil
			s.backoff = 1 * time.Second
		}
	}

	return n, err
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

	writer := newSafeWriter(logstashAddr)

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
