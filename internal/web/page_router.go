package web

import (
	"finops/internal/controllers/web"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"net/http"
	"time"
)

type PageRouterDeps struct {
	AuthService      service.AuthService
	AccoutnService   service.AccountService
	WorkspaceService service.WorkspaceService
	SessionCookie    string
	CookieSecure     bool
	RememberMeTTL    time.Duration
}

func newPageRouter(deps PageRouterDeps) http.Handler {
	mux := http.NewServeMux()

	homeController := web.NewHomeController(
		deps.AccoutnService,
		deps.WorkspaceService,
	)

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
	private.HandleFunc("GET /onboarding", onboardingController.Page)
	private.HandleFunc("POST /onboarding", onboardingController.CreateWorkspace)
	private.HandleFunc("POST /logout", authController.Logout)

	privateChain := middleware.AuthRequired(private)
	mux.Handle("/", privateChain)

	return mux
}
