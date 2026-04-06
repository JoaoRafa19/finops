package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

func AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := SessionFromContext(r.Context())

		if ok { // Encontrou a sessão
			next.ServeHTTP(w, r)
			return
		}

		
		// Acessando a pagina pelo HTMX redireciona para login
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		
		// Chamada de API retorna erro
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		
		// Inject the UserID into context if session exists
		http.Redirect(w, r.WithContext(context.WithValue(r.Context(), "user", session.UserID)), "/login", http.StatusSeeOther)
	})
}
