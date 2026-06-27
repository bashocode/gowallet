package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/bashocode/gowallet/monolith/internal/utils"
)

var Log *slog.Logger

const CorrelationIDKey = "correlation_id"

func InitLogger() {
	// set default structured JSON handler to stdout
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, // show the log level info
	})
	Log = slog.New(handler)
	slog.SetDefault(Log)
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
