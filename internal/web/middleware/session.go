package middleware

import (
	"context"
	"errors"
	"finops/internal/models"
	service "finops/internal/services"
	"net/http"
)

func SessionLoader(auth service.AuthService, cookieName string) func(http.Handler) http.Handler {
	if cookieName == "" {
		cookieName = models.DefaultAuthCookieName
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			sessionID := cookie.Value
			if sessionID == "" {
				next.ServeHTTP(w, r)
				return
			}

			session, err := auth.ValidateSession(r.Context(), sessionID)
			if err != nil {
				if errors.Is(err, service.ErrSessionNotFound) {
					next.ServeHTTP(w, r)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), models.SessionCtxKey, session)
			ctx = context.WithValue(ctx, models.SessionIDCtxKey, sessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
