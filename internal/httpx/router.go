package httpx

import (
	"finops/internal/httpx/handlers"
	"finops/internal/httpx/middleware"
	"net/http"
)



func NewRouter() http.Handler {
	mux := http.NewServeMux()


	mux.HandleFunc("/", handlers.Home)
	mux.HandleFunc("/health", handlers.Health)

	return middleware.Loggin(mux)
}