package web

import (
	accounts "finops/internal/modules/accounts/web"
	auth "finops/internal/modules/auth/web"
	category "finops/internal/modules/category/web"
	home "finops/internal/modules/home/web"
	onboarding "finops/internal/modules/onboarding/web"
	transactions "finops/internal/modules/transactions/web"
	"finops/internal/web/middleware"
	"net/http"
)

func newPageRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	homeController := home.NewController(
		deps.AccountService,
		deps.CategoryService,
		deps.WorkspaceService,
	)
	accountController := accounts.NewController(deps.AccountService)
	onboardingController := onboarding.NewController(deps.WorkspaceService)
	transactionController := transactions.NewController(
		deps.TransactionService,
		deps.AccountService,
		deps.CategoryService,
	)
	categoryController := category.NewCategoryController(deps.CategoryService)
	authController := auth.NewController(
		deps.AuthService,
		deps.SessionCookie,
		deps.CookieSecure,
		deps.RememberMeTTL,
	)

	mux.HandleFunc("GET /login", authController.LoginPage)
	mux.HandleFunc("POST /login", authController.Login)

	private := http.NewServeMux()
	private.HandleFunc("GET /", homeController.Home)
	private.HandleFunc("POST /accounts", accountController.Create)
	private.HandleFunc("GET /account-modal", accountController.AccountModal)
	private.HandleFunc("GET /accounts/{id}/edit", accountController.EditForm)
	private.HandleFunc("POST /accounts/{id}", accountController.Update)
	private.HandleFunc("GET /accounts/{id}", accountController.ShowItem)
	private.HandleFunc("GET /onboarding", onboardingController.Page)
	private.HandleFunc("POST /onboarding", onboardingController.CreateWorkspace)
	private.HandleFunc("POST /logout", authController.Logout)
	private.HandleFunc("POST /categories", categoryController.CreateCategory)
	private.HandleFunc("GET /transaction-modal", transactionController.RegisterTransactionModal)
	private.HandleFunc("POST /transactions", transactionController.CreateTransaction)

	privateChain := middleware.AuthRequired(private)
	mux.Handle("/", privateChain)

	return mux
}
