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
				http.Error(w, "Muitas tentativas. Tente novamente em alguns minutos.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

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
