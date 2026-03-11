package web

import (
	"database/sql"
	apiCtrl "finops/internal/controllers/api"
	"net/http"

	"github.com/redis/go-redis/v9"
)

func newAPIRouter(db *sql.DB, redisClient *redis.Client) http.Handler {
	mux := http.NewServeMux()

	healthController := apiCtrl.NewHealthController(db, redisClient)
	mux.HandleFunc("GET /health", healthController.Health)

	return mux
}
