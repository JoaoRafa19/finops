package middleware

import (
	"context"
	"net/http"
	"strings"
)

// EmailVerifiedChecker resolves whether the given user has verified their email.
// Kept minimal so this middleware doesn't depend on the full auth service.
type EmailVerifiedChecker interface {
	IsEmailVerified(ctx context.Context, userID int64) (bool, error)
}

// pathsAllowedWhenUnverified are the only routes an authenticated-but-unverified
// user can reach. Everything else redirects to the pending page.
var pathsAllowedWhenUnverified = map[string]struct{}{
	"/verify-email":         {},
	"/verify-email/pending": {},
	"/verify-email/resend":  {},
	"/logout":               {},
}

// EmailVerified redirects authenticated users with unverified emails to the
// verification pending page for any non-allowlisted route. Must run after
// AuthRequired.
func EmailVerified(checker EmailVerifiedChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := SessionFromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			if _, allowed := pathsAllowedWhenUnverified[r.URL.Path]; allowed {
				next.ServeHTTP(w, r)
				return
			}
			// Rotas estáticas / prefixos permitidos (nada por enquanto).
			verified, err := checker.IsEmailVerified(r.Context(), session.UserID)
			if err != nil || verified {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/verify-email/pending")
				w.WriteHeader(http.StatusOK)
				return
			}
			if strings.Contains(r.Header.Get("Accept"), "application/json") {
				http.Error(w, `{"error":"email_not_verified"}`, http.StatusForbidden)
				return
			}
			http.Redirect(w, r, "/verify-email/pending", http.StatusSeeOther)
		})
	}
}
