package service

import "errors"

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrSessionNotFound       = errors.New("session not found")
	ErrEmailTaken            = errors.New("email already registered")
	ErrInvalidOrExpiredToken = errors.New("token inválido ou expirado")
)
