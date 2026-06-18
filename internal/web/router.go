package web

import (
	"database/sql"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type RouterDeps struct {
	AuthService           service.AuthService
	AccountService        service.AccountService
	WorkspaceService      service.WorkspaceService
	CategoryService       service.CategoryService
	TransactionService    service.TransactionService
	ReportService         service.ReportsService
	ImportService         service.ImportService
	ClassificationService service.ClassificationService
	ChatService           service.ChatService
	DB                    *sql.DB
	RedisClient           *redis.Client
	SessionCookie         string
	CookieSecure          bool
	RememberMeTTL         time.Duration
}

func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/", newPageRouter(deps))
	mux.Handle("/api/", http.StripPrefix("/api", newAPIRouter(deps.DB, deps.RedisClient)))

	handler := middleware.CSRFMiddleware(deps.AuthService)(mux)
	handler = middleware.SessionLoader(deps.AuthService, deps.SessionCookie, deps.CookieSecure)(handler)
	handler = middleware.Logging(handler)
	handler = middleware.RequestID(handler)

	return handler
}
