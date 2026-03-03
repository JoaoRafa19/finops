package web

import (
	webCtrl "finops/internal/controllers/web"
	"net/http"
)

func newPageRouter() http.Handler {
	mux := http.NewServeMux()

	homeController := webCtrl.NewHomeController()
	mux.HandleFunc("GET /", homeController.Home)

	return mux
}
