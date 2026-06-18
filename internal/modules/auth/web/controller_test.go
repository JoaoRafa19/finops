package auth

import (
	"context"
	"errors"
	"finops/internal/models"
	service "finops/internal/services"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type authServiceStub struct {
	loginFn func(ctx context.Context, email, password string, rememberMe bool) (models.Session, error)
}

func (a authServiceStub) Login(ctx context.Context, email, password string, rememberMe bool) (models.Session, error) {
	return a.loginFn(ctx, email, password, rememberMe)
}

func (a authServiceStub) Logout(context.Context, string) error {
	return nil
}

func (a authServiceStub) ValidateSession(context.Context, string) (models.Session, error) {
	return models.Session{}, nil
}

func (a authServiceStub) RotateSession(context.Context, string) (models.Session, error) {
	return models.Session{}, nil
}

func (a authServiceStub) IssueCSRFToken(context.Context, string) (string, error) {
	return "", nil
}

func (a authServiceStub) ValidateCSRFToken(context.Context, string, string) (bool, error) {
	return false, nil
}

func (a authServiceStub) Register(context.Context, string, string) (models.Session, error) {
	return models.Session{}, nil
}

func TestAuthControllerLoginRedirectsAfterSuccess(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Add(30 * time.Minute)
	controller := NewController(authServiceStub{
		loginFn: func(ctx context.Context, email, password string, rememberMe bool) (models.Session, error) {
			if email != "user@example.com" || password != "secret" || !rememberMe {
				t.Fatalf("unexpected login payload: email=%q password=%q remember=%v", email, password, rememberMe)
			}

			return models.Session{
				ID:         "session-123",
				UserID:     42,
				Email:      email,
				RememberMe: rememberMe,
				ExpiresAt:  now,
			}, nil
		},
	}, "finops_session", false, 7*24*time.Hour)

	form := url.Values{
		"email":       {"user@example.com"},
		"password":    {"secret"},
		"remember_me": {"on"},
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	controller.Login(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, res.StatusCode)
	}

	if got := res.Header.Get("Location"); got != "/" {
		t.Fatalf("expected redirect to /, got %q", got)
	}

	cookie := res.Cookies()
	if len(cookie) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookie))
	}

	if cookie[0].Name != "finops_session" || cookie[0].Value != "session-123" {
		t.Fatalf("unexpected cookie: %#v", cookie[0])
	}
}

func TestAuthControllerLoginPageRedirectsAuthenticatedUser(t *testing.T) {
	t.Parallel()

	controller := NewController(authServiceStub{}, "finops_session", false, 7*24*time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req = req.WithContext(context.WithValue(req.Context(), models.SessionCtxKey, models.Session{
		ID:     "session-123",
		UserID: 42,
		Email:  "user@example.com",
	}))

	rec := httptest.NewRecorder()
	controller.LoginPage(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, res.StatusCode)
	}

	if got := res.Header.Get("Location"); got != "/" {
		t.Fatalf("expected redirect to /, got %q", got)
	}
}

func TestAuthControllerLoginReturnsUnauthorizedForInvalidCredentials(t *testing.T) {
	t.Parallel()

	controller := NewController(authServiceStub{
		loginFn: func(ctx context.Context, email, password string, rememberMe bool) (models.Session, error) {
			return models.Session{}, service.ErrInvalidCredentials
		},
	}, "finops_session", false, 7*24*time.Hour)

	form := url.Values{
		"email":    {"user@example.com"},
		"password": {"wrong"},
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	controller.Login(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.StatusCode)
	}
}

func TestAuthControllerLoginReturnsInternalServerErrorOnUnexpectedFailure(t *testing.T) {
	t.Parallel()

	controller := NewController(authServiceStub{
		loginFn: func(ctx context.Context, email, password string, rememberMe bool) (models.Session, error) {
			return models.Session{}, errors.New("redis down")
		},
	}, "finops_session", false, 7*24*time.Hour)

	form := url.Values{
		"email":    {"user@example.com"},
		"password": {"secret"},
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	controller.Login(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.StatusCode)
	}
}
