package middleware

import (
	"encoding/json"
	service "finops/internal/services"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// csrfExemptPaths são entry points públicos onde exigir CSRF token contra
// uma sessão existente causa 403 espúrio (usuário volta ao /login com Back
// button, submete de novo). Credencial já é o segredo desses endpoints.
var csrfExemptPaths = map[string]struct{}{
	"/login":  {},
	"/signup": {},
}

func CSRFMiddleware(auth service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			if _, exempt := csrfExemptPaths[r.URL.Path]; exempt {
				next.ServeHTTP(w, r)
				return
			}

			sessionID, ok := SessionIDFromContext(r.Context())

			if !ok || sessionID == "" {
				if !isSameOrigin(r) {
					writeCSRFFailure(w, r, http.StatusForbidden, "cross-origin request blocked")
					return
				}
				// Sem sessão autenticada, deixa AuthRequired decidir.
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
					_ = r.ParseMultipartForm(32 << 20)
				} else {
					_ = r.ParseForm()
				}
				token = r.FormValue("_csrf")
			}

			valid, err := auth.ValidateCSRFToken(r.Context(), sessionID, token)
			if err != nil {
				writeCSRFFailure(w, r, http.StatusInternalServerError, "CSRF Validation failed")
				return
			}

			if !valid {
				writeCSRFFailure(w, r, http.StatusForbidden, "csrf token invalid")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	u, err := url.Parse(origin)

	return err == nil && u.Hostname() == host
}

func writeCSRFFailure(w http.ResponseWriter, r *http.Request, status int, msg string) {
	if r.Header.Get("HX-Request") == "true" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte("forbidden"))
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
