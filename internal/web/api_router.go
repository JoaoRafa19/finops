package web

import (
	apiCtrl "finops/internal/controllers/api"
	"net/http"
)

func newAPIRouter() http.Handler {
	mux := http.NewServeMux()

	healthController := apiCtrl.NewHealthController()
	mux.HandleFunc("GET /health", healthController.Health)

	return mux
}

