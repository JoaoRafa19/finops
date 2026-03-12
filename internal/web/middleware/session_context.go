package middleware

import (
	"context"
	"finops/internal/models"
	"finops/internal/observability"
	"log/slog"
)

func SessionFromContext(ctx context.Context) (models.Session, bool) {
	session, ok := ctx.Value(models.SessionCtxKey).(models.Session)
	return session, ok
}

func SessionIDFromContext(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(models.SessionIDCtxKey).(string)
	return sessionID, ok
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	return observability.RequestID(ctx)
}

func LoggerFromContext(ctx context.Context) (*slog.Logger, bool) {
	logger := observability.Logger(ctx)
	return logger, logger != nil
}
