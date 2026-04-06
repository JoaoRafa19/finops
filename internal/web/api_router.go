package web

import (
	"database/sql"
	"finops/internal/modules/health/api"
	"net/http"

	"github.com/redis/go-redis/v9"
)

func newAPIRouter(db *sql.DB, redisClient *redis.Client) http.Handler {
	mux := http.NewServeMux()

	healthController := health.NewController(db, redisClient)
	mux.HandleFunc("GET /health", healthController.Health)

	return mux
}