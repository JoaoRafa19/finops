package web

import (
	"finops/internal/web/middleware"
	"net/http"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/", newPageRouter())
	mux.Handle("/api/", http.StripPrefix("/api", newAPIRouter()))

	return middleware.Logging(mux)
}
