package web

import (
	"finops/internal/web/middleware"
	"net/http"
)

func NewRouter(deps PageRouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/", newPageRouter(deps))
	mux.Handle("/api/", http.StripPrefix("/api", newAPIRouter(deps.DB, deps.RedisClient)))

	handler := middleware.CSRFMiddleware(deps.AuthService)(mux)
	handler = middleware.SessionLoader(deps.AuthService, deps.SessionCookie)(handler)
	handler = middleware.Logging(handler)
	handler = middleware.RequestID(handler)

	return handler
}
