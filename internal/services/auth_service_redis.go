package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"finops/internal/models"
	"finops/internal/observability"
	"finops/internal/store"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type RedisAuthService struct {
	rdb               *redis.Client
	queries           *store.Queries
	sessionTTL        time.Duration
	rememberMeTTL     time.Duration
	slidingSessionTTL bool
}

// NewRedisAuthService cria o serviço de autenticação stateful.
// A sessão é persistida no Redis e identificada por um session_id no cookie.
func NewRedisAuthService(rdb *redis.Client,
	queries *store.Queries,
	sessionTTL time.Duration,
	rememberMeTTL time.Duration,
	slidingSessionTTL bool) *RedisAuthService {
	return &RedisAuthService{
		rdb:               rdb,
		queries:           queries,
		sessionTTL:        sessionTTL,
		rememberMeTTL:     rememberMeTTL,
		slidingSessionTTL: slidingSessionTTL,
	}
}

// IssueCSRFToken rotaciona o token CSRF de uma sessão existente.
// Mantemos o mesmo TTL remanescente da sessão para não alterar sua janela de validade.
func (s *RedisAuthService) IssueCSRFToken(ctx context.Context, sessionID string) (string, error) {
	logger := observability.Logger(ctx)
	session, err := s.getSession(ctx, sessionID)
	if err != nil {
		logger.Error("auth_issue_csrf_load_session_failed", "error", err)
		return "", err
	}

	token, err := randomToken(32)
	if err != nil {
		logger.Error("auth_issue_csrf_generate_failed", "user_id", session.UserID, "error", err)
		return "", err
	}

	session.CSRFToken = token
	ttl, err := s.rdb.TTL(ctx, sessionKey(sessionID)).Result()
	if err != nil {
		logger.Error("auth_issue_csrf_ttl_failed", "user_id", session.UserID, "error", err)
		return "", fmt.Errorf("read session ttl: %w", err)
	}

	if ttl <= 0 {
		if session.RememberMe {
			ttl = s.rememberMeTTL
		} else {
			ttl = s.sessionTTL
		}
	}

	if err := s.saveSession(ctx, session, ttl); err != nil {
		logger.Error("auth_issue_csrf_save_failed", "user_id", session.UserID, "error", err)
		return "", err
	}

	logger.Info("auth_issue_csrf_succeeded", "user_id", session.UserID)
	return token, nil
}

// RotateSession invalida a sessão atual e emite uma nova, preservando usuário e remember_me.
// Útil para mitigar fixation e trocar credenciais de sessão após eventos sensíveis.
func (s *RedisAuthService) RotateSession(ctx context.Context, sessionID string) (models.Session, error) {
	logger := observability.Logger(ctx)
	current, err := s.getSession(ctx, sessionID)
	if err != nil {
		logger.Error("auth_rotate_session_load_failed", "error", err)
		return models.Session{}, err
	}

	if err := s.Logout(ctx, sessionID); err != nil {
		logger.Error("auth_rotate_session_revoke_failed", "user_id", current.UserID, "error", err)
		return models.Session{}, err
	}
	newSession, ttl, err := s.newSession(current.UserID, current.Email, current.RememberMe)
	if err != nil {
		logger.Error("auth_rotate_session_generate_failed", "user_id", current.UserID, "error", err)
		return models.Session{}, err
	}
	if err := s.saveSession(ctx, newSession, ttl); err != nil {
		logger.Error("auth_rotate_session_save_failed", "user_id", current.UserID, "error", err)
		return models.Session{}, err
	}

	logger.Info("auth_rotate_session_succeeded", "user_id", current.UserID, "remember_me", current.RememberMe)
	return newSession, nil
}

// ValidateCSRFToken compara o token enviado pela requisição com o token salvo na sessão.
// A comparação é feita em tempo constante para reduzir risco de side-channel.
func (s *RedisAuthService) ValidateCSRFToken(ctx context.Context, sessionID string, token string) (bool, error) {
	logger := observability.Logger(ctx)
	session, err := s.getSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			logger.Warn("auth_validate_csrf_session_not_found")
			return false, nil
		}
		logger.Error("auth_validate_csrf_load_failed", "error", err)
		return false, err
	}

	if token == "" || session.CSRFToken == "" {
		logger.Warn("auth_validate_csrf_missing_token", "user_id", session.UserID)
		return false, nil
	}

	valid := subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(token)) == 1
	if !valid {
		logger.Warn("auth_validate_csrf_invalid", "user_id", session.UserID)
	}

	return valid, nil

}

// ValidateSession valida a existência da sessão e opcionalmente renova TTL (sliding expiration).
// A renovação acontece somente quando slidingSessionTTL está habilitado.
func (s *RedisAuthService) ValidateSession(ctx context.Context, sessionID string) (models.Session, error) {
	logger := observability.Logger(ctx)
	session, err := s.getSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			logger.Warn("auth_validate_session_not_found")
		} else {
			logger.Error("auth_validate_session_load_failed", "error", err)
		}
		return models.Session{}, err
	}

	if s.slidingSessionTTL { // Renova o TTL
		ttl := s.sessionTTL
		if session.RememberMe {
			ttl = s.rememberMeTTL
		}

		session.LastSeenAt = time.Now().UTC()
		session.ExpiresAt = session.LastSeenAt.Add(ttl)

		if err := s.saveSession(ctx, session, ttl); err != nil {
			logger.Error("auth_validate_session_refresh_failed", "user_id", session.UserID, "error", err)
			return models.Session{}, err
		}
	}
	logger.Debug("auth_validate_session_succeeded", "user_id", session.UserID, "remember_me", session.RememberMe)
	return session, nil
}

// Logout remove a sessão do Redis (revogação imediata server-side).
func (s *RedisAuthService) Logout(ctx context.Context, sessionID string) error {
	logger := observability.Logger(ctx)
	if err := s.rdb.Del(ctx, sessionKey(sessionID)).Err(); err != nil {
		logger.Error("auth_logout_failed", "error", err)
		return fmt.Errorf("delete session: %w", err)
	}

	logger.Info("auth_logout_succeeded")
	return nil
}

// Login valida credenciais do usuário e cria uma nova sessão autenticada.
// O hash de senha é verificado com bcrypt.
func (s *RedisAuthService) Login(ctx context.Context, email, password string, rememberme bool) (models.Session, error) {
	logger := observability.Logger(ctx)
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		logger.Warn("auth_login_invalid_credentials_lookup", "remember_me", rememberme)
		return models.Session{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		logger.Warn("auth_login_invalid_credentials_password", "remember_me", rememberme)
		return models.Session{}, ErrInvalidCredentials
	}

	session, ttl, err := s.newSession(user.ID, email, rememberme)
	if err != nil {
		logger.Error("auth_login_new_session_failed", "user_id", user.ID, "error", err)
		return models.Session{}, err
	}

	if err := s.saveSession(ctx, session, ttl); err != nil {
		logger.Error("auth_login_save_session_failed", "user_id", user.ID, "error", err)
		return models.Session{}, err
	}

	logger.Info("auth_login_succeeded", "user_id", user.ID, "remember_me", rememberme)
	return session, nil
}

// newSession gera um session_id e CSRF token criptograficamente seguros.
// O TTL varia entre sessão normal e remember_me.
func (s *RedisAuthService) newSession(uid int64, email string, rememberMe bool) (models.Session, time.Duration, error) {
	sessionId, err := randomToken(32)
	if err != nil {
		return models.Session{}, 0, err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return models.Session{}, 0, err
	}

	now := time.Now().UTC()
	ttl := s.sessionTTL

	if rememberMe {
		ttl = s.rememberMeTTL
	}

	session := models.Session{
		ID:         sessionId,
		UserID:     uid,
		Email:      email,
		CSRFToken:  csrfToken,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(ttl),
		RememberMe: rememberMe,
	}

	return session, ttl, nil
}

// randomToken retorna um token hexadecimal baseado em bytes criptograficamente aleatórios.
func randomToken(i int) (string, error) {
	buf := make([]byte, i)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}

	return hex.EncodeToString(buf), nil
}

// getSession resolve a sessão pelo ID a partir do Redis.
func (s *RedisAuthService) getSession(ctx context.Context, sessionID string) (models.Session, error) {
	payload, err := s.rdb.Get(ctx, sessionKey(sessionID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return models.Session{}, ErrSessionNotFound
		}
		return models.Session{}, fmt.Errorf("get session: %w", err)
	}

	var session models.Session

	if err := json.Unmarshal([]byte(payload), &session); err != nil {
		return models.Session{}, fmt.Errorf("decode session: %w", err)
	}

	return session, err
}

// saveSession serializa a sessão em JSON e persiste com TTL explícito no Redis.
func (s *RedisAuthService) saveSession(ctx context.Context, session models.Session, ttl time.Duration) error {
	raw, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}

	if err := s.rdb.Set(ctx, sessionKey(session.ID), raw, ttl).Err(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

// sessionKey centraliza o padrão de chave usado no Redis.
func sessionKey(sessionId string) string {
	return "sess:" + sessionId
}
