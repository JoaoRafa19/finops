package middleware

import (
	"log"
	"net/http"
	"time"
)


func Loggin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w,r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}