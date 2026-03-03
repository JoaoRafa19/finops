package web

import (
	"finops/internal/controllers/web"
	webCtrl "finops/internal/controllers/web"
	"net/http"
)

func newPageRouter() http.Handler {
	mux := http.NewServeMux()

	homeController := webCtrl.NewHomeController()

	
	mux.Handle("/", homeRouter(*homeController))

	return mux
}

func homeRouter(ctrl web.HomeController) http.Handler {
	// All Home Routes
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", ctrl.Home)

	return mux
}