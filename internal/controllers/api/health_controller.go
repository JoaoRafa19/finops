package api

import (
	"database/sql"
	"finops/internal/utils"
	"net/http"

	"github.com/redis/go-redis/v9"
)

type HealthController struct {
	db    *sql.DB
	redis *redis.Client
}

func NewHealthController(db *sql.DB, rdb *redis.Client) *HealthController {
	return &HealthController{
		db:    db,
		redis: rdb,
	}
}

func (c *HealthController) Health(w http.ResponseWriter, r *http.Request) {
	if err := c.db.PingContext(r.Context()); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "error connecting database"})
		return
	}

	if _, err := c.redis.Ping(r.Context()).Result(); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "error connecting redis"})
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}
