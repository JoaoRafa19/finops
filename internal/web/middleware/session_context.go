package middleware

import (
	"context"
	"finops/internal/models"
)

func SessionFromContext(ctx context.Context) (models.Session, bool) {
	session, ok := ctx.Value(models.SessionCtxKey).(models.Session)
	return session, ok
}

func SessionIDFromContext(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(models.SessionIDCtxKey).(string)
	return sessionID, ok
}
