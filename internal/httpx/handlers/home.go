package handlers

import "net/http"

func Home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_,_ = w.Write([]byte("fiops online"))
}
