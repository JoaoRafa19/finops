package observability

import (
	"context"
	"finops/internal/models"
	"log/slog"
)

func WithRequestContext(ctx context.Context, requestID string) context.Context {
	logger := slog.Default()
	if requestID != "" {
		logger = logger.With(slog.String("request_id", requestID))
		ctx = context.WithValue(ctx, models.RequestIDCtxKey, requestID)
	}

	return context.WithValue(ctx, models.LoggerCtxKey, logger)
}

func Logger(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}

	logger, ok := ctx.Value(models.LoggerCtxKey).(*slog.Logger)
	if !ok || logger == nil {
		return slog.Default()
	}

	return logger
}

func RequestID(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}

	requestID, ok := ctx.Value(models.RequestIDCtxKey).(string)
	return requestID, ok
}
