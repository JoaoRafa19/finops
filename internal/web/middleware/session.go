package middleware

import (
	"context"
	"errors"
	"finops/internal/models"
	service "finops/internal/services"
	"net/http"
	"time"
)

func SessionLoader(auth service.AuthService, cookieName string, cookieSecure bool) func(http.Handler) http.Handler {
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

			effectiveSessionID := sessionID
			effectiveSession := session 


			if time.Until(session.ExpiresAt) < 5*time.Minute {
				newSession, err := auth.RotateSession(r.Context(), sessionID)
				if err == nil {
					effectiveSessionID = newSession.ID 
					effectiveSession = newSession

					http.SetCookie(w, &http.Cookie{
						Name: cookieName,
						Value: newSession.ID,
						Path: "/",
						HttpOnly: true,
						Secure: cookieSecure,
						SameSite: http.SameSiteLaxMode,
						Expires: newSession.ExpiresAt,
						MaxAge: int(time.Until(newSession.ExpiresAt).Seconds()),
					})
				}
			}

			ctx := context.WithValue(r.Context(), models.SessionCtxKey, effectiveSession)
			ctx = context.WithValue(ctx, models.SessionIDCtxKey, effectiveSessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
