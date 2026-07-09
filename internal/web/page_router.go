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
	profile "finops/internal/modules/profile/web"
	projections "finops/internal/modules/projections/web"
	reports "finops/internal/modules/reports/web"
	transactions "finops/internal/modules/transactions/web"
	"finops/internal/web/middleware"
	"net/http"
	"time"
)

func newPageRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	homeController := home.NewController(
		deps.AccountService,
		deps.WorkspaceService,
		deps.ReportService,
		deps.ImportService,
		deps.TourService,
		deps.CategoryService,
		deps.UserSettingsService,
	)
	accountController := accounts.NewController(deps.AccountService)
	onboardingController := onboarding.NewController(deps.WorkspaceService, deps.UserSettingsService)
	transactionController := transactions.NewController(
		deps.TransactionService,
		deps.AccountService,
		deps.CategoryService,
	)
	categoryController := category.NewCategoryController(deps.CategoryService)
	reportsController := reports.NewReportsController(deps.ReportService, deps.CategoryService)
	projectionsController := projections.NewProjectionsController(deps.ProjectionService, deps.AccountService)
	importController := imports.NewImportController(deps.ImportService, deps.AccountService, deps.InvoiceService)
	classificationController := classification.NewClassificationController(deps.ClassificationService, deps.CategoryService)
	chatController := chat.NewChatController(deps.ChatService)
	tourController := tour.NewTourController(deps.TourService)
	profileController := profile.NewController(deps.CategoryService, deps.AccountService)
	authController := auth.NewController(
		deps.AuthService,
		deps.SessionCookie,
		deps.CookieSecure,
		deps.RememberMeTTL,
	)

	loginRL := middleware.RateLimit(deps.RedisClient, "login", 10, 5*time.Minute)
	signupRL := middleware.RateLimit(deps.RedisClient, "signup", 5, time.Hour)
	forgotRL := middleware.RateLimit(deps.RedisClient, "forgot", 5, time.Hour)
	resetRL := middleware.RateLimit(deps.RedisClient, "reset", 10, time.Hour)

	mux.HandleFunc("GET /login", authController.LoginPage)
	mux.Handle("POST /login", loginRL(http.HandlerFunc(authController.Login)))
	mux.Handle("POST /signup", signupRL(http.HandlerFunc(authController.Signup)))
	mux.HandleFunc("GET /forgot-password", authController.ForgotPasswordPage)
	mux.Handle("POST /forgot-password", forgotRL(http.HandlerFunc(authController.ForgotPassword)))
	mux.HandleFunc("GET /reset-password", authController.ResetPasswordPage)
	mux.Handle("POST /reset-password", resetRL(http.HandlerFunc(authController.ResetPassword)))
	verifyRL := middleware.RateLimit(deps.RedisClient, "verify", 20, time.Hour)
	mux.Handle("GET /verify-email", verifyRL(http.HandlerFunc(authController.VerifyEmail)))

	private := http.NewServeMux()
	private.HandleFunc("GET /", homeController.Home)
	private.HandleFunc("GET /dashboard", homeController.Dashboard)
	private.HandleFunc("POST /home/mode", homeController.ToggleMode)
	private.HandleFunc("POST /accounts", accountController.Create)
	private.HandleFunc("GET /account-modal", accountController.AccountModal)
	private.HandleFunc("GET /accounts/{id}/edit", accountController.EditForm)
	private.HandleFunc("GET /accounts/{id}/delete-modal", accountController.DeleteModal)
	private.HandleFunc("POST /accounts/{id}", accountController.Update)
	private.HandleFunc("DELETE /accounts/{id}", accountController.Delete)
	private.HandleFunc("GET /accounts/{id}", accountController.ShowItem)
	private.HandleFunc("GET /onboarding", onboardingController.Page)
	private.HandleFunc("POST /onboarding", onboardingController.CreateWorkspace)
	private.HandleFunc("POST /logout", authController.Logout)
	private.HandleFunc("GET /verify-email/pending", authController.VerifyEmailPending)
	// prefix "verify-resend2" para "resetar" a chave — a versão anterior
	// ("verify-resend") pode estar com contador max no Redis; renomear zera.
	resendVerifyRL := middleware.RateLimit(deps.RedisClient, "verify-resend2", 20, time.Hour)
	private.Handle("POST /verify-email/resend", resendVerifyRL(http.HandlerFunc(authController.ResendVerification)))
	private.HandleFunc("POST /categories", categoryController.CreateCategory)
	private.HandleFunc("GET /categories/{id}/edit", categoryController.EditRow)
	private.HandleFunc("GET /categories/{id}/row", categoryController.Row)
	private.HandleFunc("POST /categories/{id}", categoryController.UpdateCategory)
	private.HandleFunc("DELETE /categories/{id}/delete", categoryController.DeleteCategory)
	private.HandleFunc("GET /category-modal", categoryController.CategoryModal)
	private.HandleFunc("GET /transaction-modal", transactionController.RegisterTransactionModal)
	private.HandleFunc("GET /transfer-modal", transactionController.RegisterTransferModal)
	private.HandleFunc("POST /transactions", transactionController.CreateTransaction)
	private.HandleFunc("GET /transactions/{id}/edit-modal", transactionController.EditModal)
	private.HandleFunc("PUT /transactions/{id}", transactionController.UpdateTransaction)
	private.HandleFunc("DELETE /transactions/{id}", transactionController.DeleteTransaction)
	private.HandleFunc("POST /transfers", transactionController.CreateTransfer)
	private.HandleFunc("GET /reports", reportsController.Page)
	private.HandleFunc("GET /reports/spending", reportsController.Spending)
	private.HandleFunc("GET /reports/comparison", reportsController.Comparison)
	private.HandleFunc("GET /reports/balance", reportsController.Balance)
	private.HandleFunc("GET /reports/transactions", reportsController.Transactions)

	private.HandleFunc("GET /projections", projectionsController.Page)
	private.HandleFunc("GET /projections/forecast", projectionsController.Forecast)
	private.HandleFunc("GET /projections/summary", projectionsController.Summary)
	private.HandleFunc("GET /projections/simulator", projectionsController.Simulator)
	private.HandleFunc("POST /projections/variable", projectionsController.SetVariableOverride)
	private.HandleFunc("GET /projections/commitments", projectionsController.CommitmentsFragment)
	private.HandleFunc("POST /projections/commitments", projectionsController.CreateCommitment)
	private.HandleFunc("GET /projections/commitments/{id}/edit", projectionsController.EditRow)
	private.HandleFunc("GET /projections/commitments/{id}/row", projectionsController.Row)
	private.HandleFunc("POST /projections/commitments/{id}", projectionsController.UpdateCommitment)
	private.HandleFunc("DELETE /projections/commitments/{id}", projectionsController.DeleteCommitment)
	private.HandleFunc("POST /projections/settings", projectionsController.UpdateSettings)
	private.HandleFunc("GET /import", importController.Page)
	private.HandleFunc("POST /import/upload", importController.Upload)
	private.HandleFunc("POST /import/preview-csv", importController.PreviewCSV)
	private.HandleFunc("POST /import/confirm", importController.Confirm)
	private.HandleFunc("POST /import/invoice-confirm", importController.ConfirmInvoice)
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
	private.HandleFunc("POST /chat/clear", chatController.Clear)
	private.HandleFunc("GET /profile", profileController.Page)
	private.HandleFunc("GET /tour", tourController.Page)
	private.HandleFunc("GET /tour/start", tourController.Start)
	private.HandleFunc("GET /tour/complete", tourController.Complete)
	private.HandleFunc("GET /tour/skip", tourController.Skip)

	privateChain := middleware.AuthRequired(middleware.EmailVerified(deps.AuthService)(private))
	mux.Handle("/", privateChain)

	return mux
}
