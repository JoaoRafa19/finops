package auth

import (
	"errors"
	"finops/internal/models"
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Controller struct {
	auth          service.AuthService
	cookieName    string
	cookieSecure  bool
	rememberMeTTL time.Duration
}

func NewController(auth service.AuthService, cookieName string, cookieSecure bool, rememberMeTTL time.Duration) *Controller {
	if cookieName == "" {
		cookieName = models.DefaultAuthCookieName
	}

	return &Controller{
		auth:          auth,
		cookieName:    cookieName,
		cookieSecure:  cookieSecure,
		rememberMeTTL: rememberMeTTL,
	}
}

func (c *Controller) LoginPage(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	if _, ok := middleware.SessionFromContext(r.Context()); ok {
		logger.Debug("login_page_redirect_authenticated")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	errMsg := ""
	if r.URL.Query().Get("error") == "1" {
		errMsg = "usuario ou senha invalidos"
	}

	render.Templ(w, r, http.StatusOK, templates.LoginPage(errMsg))
}

func (c *Controller) Login(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	if err := r.ParseForm(); err != nil {
		logger.Warn("login_parse_form_failed", "error", err)
		render.Templ(w, r, http.StatusUnauthorized, templates.LoginPage("Erro Interno"))
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	rememberMe := r.FormValue("remember_me") == "on"

	session, err := c.auth.Login(r.Context(), email, password, rememberMe)
	slog.Debug("login_attempt", "email", email, "remember_me", rememberMe, "error", err)
	slog.Debug("login_session_issued", "session_id", session.ID, "user_id", session.UserID, "remember_me", session.RememberMe)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			logger.Warn("login_invalid_credentials", "remember_me", rememberMe)
			if r.Header.Get("HX-Request") == "true" {
				render.Templ(w, r, http.StatusOK, templates.LoginForm("usuario ou senha invalidos."))
				return
			}
			render.Templ(w, r, http.StatusUnauthorized, templates.LoginPage("usuario ou senha invalidos."))
			return
		}
		logger.Error("login_failed", "remember_me", rememberMe, "error", err)
		render.Templ(w, r, http.StatusInternalServerError, templates.LoginPage("erro no login"))
		return
	}

	c.setSessionCookie(w, session)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *Controller) Signup(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	if err := r.ParseForm(); err != nil {
		render.Templ(w, r, http.StatusBadRequest, templates.SignupForm("Erro ao processar formulário."))
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	if email == "" || password == "" {
		render.Templ(w, r, http.StatusOK, templates.SignupForm("Email e senha são obrigatórios."))
		return
	}
	if len(password) < 8 {
		render.Templ(w, r, http.StatusOK, templates.SignupForm("A senha deve ter ao menos 8 caracteres."))
		return
	}
	if password != confirm {
		render.Templ(w, r, http.StatusOK, templates.SignupForm("As senhas não coincidem."))
		return
	}

	session, err := c.auth.Register(r.Context(), email, password)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			logger.Warn("signup_email_taken", "email", email)
			render.Templ(w, r, http.StatusOK, templates.SignupForm("Este email já está cadastrado."))
			return
		}
		logger.Error("signup_failed", "error", err)
		render.Templ(w, r, http.StatusOK, templates.SignupForm("Erro ao criar conta. Tente novamente."))
		return
	}

	c.setSessionCookie(w, session)
	logger.Info("signup_succeeded", "user_id", session.UserID)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *Controller) Logout(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	sessionID, ok := middleware.SessionIDFromContext(r.Context())
	if ok && sessionID != "" {
		if err := c.auth.Logout(r.Context(), sessionID); err != nil {
			logger.Error("logout_failed", "error", err)
		}
	}

	c.clearSessionCookie(w)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (c *Controller) setSessionCookie(w http.ResponseWriter, session models.Session) {
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     c.cookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
		MaxAge:   maxAge,
	})
}

func (c *Controller) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   c.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}
