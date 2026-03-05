package web

import (
	"errors"
	"finops/internal/models"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/templates"
	"net/http"
	"time"

	"github.com/a-h/templ"
)

type AuthController struct {
	auth          service.AuthService
	cookieName    string
	cookieSecure  bool
	rememberMeTTL time.Duration
}

func NewAuthController(auth service.AuthService, cookieName string, cookieSecure bool, rememberMeTTL time.Duration) *AuthController {
	if cookieName == "" {
		cookieName = models.DefaultAuthCookieName
	}

	return &AuthController{
		auth:          auth,
		cookieName:    cookieName,
		cookieSecure:  cookieSecure,
		rememberMeTTL: rememberMeTTL,
	}
}
func renderTempl(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

func (c *AuthController) LoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.SessionFromContext(r.Context()); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	renderTempl(w, r, http.StatusOK, templates.LoginPage(""))
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderTempl(w, r, http.StatusUnauthorized, templates.LoginPage("usuario ou senha invalidos"))
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	rememberMe := r.FormValue("remember_me") == "on"

	session, err := c.auth.Login(r.Context(), email, password, rememberMe)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			renderTempl(w, r, http.StatusUnauthorized, templates.LoginPage("usuario ou senha invalidos"))
			return
		}
		renderTempl(w, r, http.StatusUnauthorized, templates.LoginPage("usuario ou senha invalidos"))
		return
	}

	c.setSessionCookie(w, session)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := middleware.SessionIDFromContext(r.Context())
	if ok && sessionID != "" {
		_ = c.auth.Logout(r.Context(), sessionID)
	}

	c.clearSessionCookie(w)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (c *AuthController) setSessionCookie(w http.ResponseWriter, session models.Session) {
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

func (c *AuthController) clearSessionCookie(w http.ResponseWriter) {
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
