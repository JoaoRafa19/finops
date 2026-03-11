package web

import (
	"database/sql"
	"finops/internal/controllers/web"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type PageRouterDeps struct {
	AuthService      service.AuthService
	AccountService   service.AccountService
	WorkspaceService service.WorkspaceService
	DB               *sql.DB
	RedisClient      *redis.Client
	SessionCookie    string
	CookieSecure     bool
	RememberMeTTL    time.Duration
}

func newPageRouter(deps PageRouterDeps) http.Handler {
	mux := http.NewServeMux()

	homeController := web.NewHomeController(
		deps.AccountService,
		deps.WorkspaceService,
	)
	accountController := web.NewAccountController(deps.AccountService)

	onboardingController := web.NewOnboardingController(deps.WorkspaceService)

	authController := web.NewAuthController(
		deps.AuthService,
		deps.SessionCookie,
		deps.CookieSecure,
		deps.RememberMeTTL,
	)

	// Públicas
	mux.HandleFunc("GET /login", authController.LoginPage)
	mux.HandleFunc("POST /login", authController.Login)

	// Privadas
	private := http.NewServeMux()
	private.HandleFunc("GET /", homeController.Home)
	private.HandleFunc("POST /accounts", accountController.Create)
	private.HandleFunc("GET /onboarding", onboardingController.Page)
	private.HandleFunc("POST /onboarding", onboardingController.CreateWorkspace)
	private.HandleFunc("POST /logout", authController.Logout)

	privateChain := middleware.AuthRequired(private)
	mux.Handle("/", privateChain)

	return mux
}
