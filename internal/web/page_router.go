package web

import (
	accounts "finops/internal/modules/accounts/web"
	auth "finops/internal/modules/auth/web"
	category "finops/internal/modules/category/web"
	chat "finops/internal/modules/chat/web"
	tour "finops/internal/modules/tour/web"
	classification "finops/internal/modules/classification/web"
	home "finops/internal/modules/home/web"
	imports "finops/internal/modules/imports/web"
	onboarding "finops/internal/modules/onboarding/web"
	reports "finops/internal/modules/reports/web"
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
		deps.TransactionService,
		deps.ImportService,
		deps.TourService,
	)
	accountController := accounts.NewController(deps.AccountService)
	onboardingController := onboarding.NewController(deps.WorkspaceService)
	transactionController := transactions.NewController(
		deps.TransactionService,
		deps.AccountService,
		deps.CategoryService,
	)
	categoryController := category.NewCategoryController(deps.CategoryService)
	reportsController := reports.NewReportsController(deps.ReportService)
	importController := imports.NewImportController(deps.ImportService, deps.AccountService)
	classificationController := classification.NewClassificationController(deps.ClassificationService, deps.CategoryService)
	chatController := chat.NewChatController(deps.ChatService)
	tourController := tour.NewTourController(deps.TourService)
	authController := auth.NewController(
		deps.AuthService,
		deps.SessionCookie,
		deps.CookieSecure,
		deps.RememberMeTTL,
	)

	mux.HandleFunc("GET /login", authController.LoginPage)
	mux.HandleFunc("POST /login", authController.Login)
	mux.HandleFunc("POST /signup", authController.Signup)
	mux.HandleFunc("GET /forgot-password", authController.ForgotPasswordPage)
	mux.HandleFunc("POST /forgot-password", authController.ForgotPassword)
	mux.HandleFunc("GET /reset-password", authController.ResetPasswordPage)
	mux.HandleFunc("POST /reset-password", authController.ResetPassword)

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
	private.HandleFunc("DELETE /categories/{id}/delete", categoryController.DeleteCategory)
	private.HandleFunc("GET /category-modal", categoryController.CategoryModal)
	private.HandleFunc("GET /transaction-modal", transactionController.RegisterTransactionModal)
	private.HandleFunc("GET /transfer-modal", transactionController.RegisterTransferModal)
	private.HandleFunc("POST /transactions", transactionController.CreateTransaction)
	private.HandleFunc("POST /transfers", transactionController.CreateTransfer)
	private.HandleFunc("GET /reports", reportsController.Page)
	private.HandleFunc("GET /reports/spending", reportsController.Spending)
	private.HandleFunc("GET /reports/comparison", reportsController.Comparison)
	private.HandleFunc("GET /reports/balance", reportsController.Balance)
	private.HandleFunc("GET /reports/transactions", reportsController.Transactions)
	private.HandleFunc("GET /import", importController.Page)
	private.HandleFunc("POST /import/upload", importController.Upload)
	private.HandleFunc("POST /import/preview-csv", importController.PreviewCSV)
	private.HandleFunc("POST /import/confirm", importController.Confirm)
	private.HandleFunc("GET /classify", classificationController.Page)
	private.HandleFunc("GET /classify/{id}/modal", classificationController.SuggestModal)
	private.HandleFunc("POST /classify", classificationController.Classify)
	private.HandleFunc("GET /classify/search", classificationController.SearchCategories)
	private.HandleFunc("POST /classify/create-and-classify", classificationController.CreateAndClassify)
	private.HandleFunc("GET /notifications/count", classificationController.NotificationsCount)
	private.HandleFunc("POST /classify/auto", classificationController.AutoClassify)
	private.HandleFunc("POST /classify/bulk-confirm", classificationController.BulkConfirm)
	private.HandleFunc("POST /chat", chatController.Message)
	private.HandleFunc("GET /chat/history", chatController.GetHistory)
	private.HandleFunc("GET /tour", tourController.Page)
	private.HandleFunc("GET /tour/start", tourController.Start)
	private.HandleFunc("GET /tour/complete", tourController.Complete)
	private.HandleFunc("GET /tour/skip", tourController.Skip)

	privateChain := middleware.AuthRequired(private)
	mux.Handle("/", privateChain)

	return mux
}
