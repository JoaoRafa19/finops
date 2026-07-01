package middleware

import (
	"finops/internal/observability"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimit returns middleware that limits requests per client IP using a Redis
// fixed-window counter. Bucket is scoped by `prefix` so different endpoints
// don't share counters.
//
// ponytail: fixed window (INCR + EXPIRE). Migrate to sliding window / token
// bucket if login attacks stagger across window boundaries.
func RateLimit(rdb *redis.Client, prefix string, max int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			key := fmt.Sprintf("finops:rl:%s:%s", prefix, ip)

			count, err := rdb.Incr(r.Context(), key).Result()
			if err != nil {
				observability.Logger(r.Context()).Error("ratelimit_redis_failed", "error", err, "prefix", prefix)
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				_ = rdb.Expire(r.Context(), key, window).Err()
			}
			if count > int64(max) {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				if r.Header.Get("HX-Request") == "true" {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte(`<div class="rounded-xl bg-amber-50 p-4 text-sm text-amber-800"><i class="fa-solid fa-clock mr-2"></i>Muitas tentativas. Tente novamente em alguns minutos.</div>`))
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(rateLimitPageHTML))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

const rateLimitPageHTML = `<!DOCTYPE html>
<html lang="pt-BR"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Muitas tentativas · Finops</title>
<script src="https://cdn.tailwindcss.com"></script>
<link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.2/css/all.min.css" rel="stylesheet">
</head><body class="min-h-screen bg-slate-100 font-sans text-slate-900 flex items-center justify-center px-4">
<div class="max-w-md w-full rounded-2xl border border-slate-200 bg-white p-8 shadow-lg text-center">
<div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-100 text-2xl text-amber-600"><i class="fa-solid fa-clock"></i></div>
<h1 class="text-2xl font-extrabold">Muitas tentativas</h1>
<p class="mt-3 text-sm text-slate-600">Você excedeu o limite temporário de requisições. Aguarde alguns minutos e tente novamente.</p>
<a href="/login" class="mt-6 inline-flex items-center justify-center rounded-xl bg-slate-900 px-6 py-3 text-sm font-bold text-white transition hover:bg-slate-800">Voltar ao login</a>
</div></body></html>`

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
