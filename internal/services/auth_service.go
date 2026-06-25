package service

import (
	"context"
	"finops/internal/models"
)

type AuthService interface {
	Login(ctx context.Context, email, password string, rememberMe bool) (models.Session, error)
	Register(ctx context.Context, email, password string) (models.Session, error)
	Logout(ctx context.Context, sessionID string) error
	ValidateSession(ctx context.Context, sessionID string) (models.Session, error)
	RotateSession(ctx context.Context, sessionID string) (models.Session, error)
	IssueCSRFToken(ctx context.Context, sessionID string) (string, error)
	ValidateCSRFToken(ctx context.Context, sessionID, token string) (bool, error)
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
}
