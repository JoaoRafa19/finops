package app

import (
	"context"
	"database/sql"
	"log/slog"
	service "finops/internal/services"
	"finops/internal/store"
	"finops/internal/worker"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Services struct {
	Auth           service.AuthService
	Account        service.AccountService
	Workspace      service.WorkspaceService
	Transaction    service.TransactionService
	Category       service.CategoryService
	Report         service.ReportsService
	Import         service.ImportService
	Classification service.ClassificationService
	Chat           service.ChatService
	Tour           service.TourService
	Projection     service.ProjectionService
	Invoice        service.InvoiceService
	UserSettings   service.UserSettingsService
}

type Runtime struct {
	Config      Config
	DB          *sql.DB
	RedisClient *redis.Client
	Services    Services

	stopWorker context.CancelFunc
}

// NewEmailSender escolhe o sender real conforme a config: Resend > SMTP > Noop.
// Usado pelo worker embutido e pelo binário standalone cmd/email-worker.
func NewEmailSender(cfg Config) service.EmailService {
	switch {
	case cfg.ResendAPIKey != "":
		slog.Info("email_service_resend", "from", cfg.ResendFrom)
		return service.NewResendEmailService(cfg.ResendAPIKey, cfg.ResendFrom)
	case cfg.SMTPHost != "":
		slog.Info("email_service_smtp", "host", cfg.SMTPHost, "port", cfg.SMTPPort, "from", cfg.SMTPFrom)
		return service.NewSMTPEmailService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPFrom)
	default:
		slog.Warn("email_service_noop",
			"hint", "set RESEND_API_KEY (recommended in prod) or SMTP_HOST/PORT/USER/PASSWORD/FROM to enable e-mail sending")
		return service.NewNoopEmailService()
	}
}

func Bootstrap(ctx context.Context, cfg Config) (*Runtime, error) {
	db, err := NewDB(ctx, cfg.DbURL)
	if err != nil {
		return nil, err
	}

	if cfg.MigrateOnStart {
		slog.Info("migrate_on_start_running")
		if err := store.MigrateUp(ctx, db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("migrate on start: %w", err)
		}
		slog.Info("migrate_on_start_done")
	} else {
		slog.Warn("migrate_on_start_disabled",
			"hint", "set MIGRATE_ON_START=true to auto-apply pending migrations")
	}

	queries := store.New(db)
	if queries == nil {
		_ = db.Close()
		return nil, fmt.Errorf("queries initialization failed")
	}

	redisClient, err := NewRedisClient(ctx, cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	embSvc := service.NewEmbeddingService(cfg.EmbeddingBaseURL, cfg.EmbeddingAPIKey, cfg.EmbeddingModel)

	// Handlers empilham no Redis; o worker embutido consome e envia de fato.
	// O worker ganha ctx próprio (o ctx do Bootstrap tem timeout de 10s) e é
	// parado no Runtime.Close.
	emailSvc := service.NewQueueEmailService(redisClient)
	workerCtx, stopWorker := context.WithCancel(context.Background())
	go worker.RunEmailWorker(workerCtx, redisClient, NewEmailSender(cfg))

	projectionSvc := service.NewPGProjectionService(queries)
	importSvc := service.NewPGImportService(db, queries)
	invoiceSvc := service.NewAIInvoiceService(
		queries,
		service.NewAIService(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel),
		importSvc,
		projectionSvc,
	)

	services := Services{
		Auth: service.NewRedisAuthService(
			redisClient,
			queries,
			emailSvc,
			cfg.AppBaseURL,
			cfg.SessionTTL,
			cfg.RememberMeTTL,
			cfg.SlidingSessionTTL,
		),
		Account:     service.NewPGAccountService(db, queries),
		Workspace:   service.NewPGWorkspaceService(queries),
		Transaction: service.NewPGTransactionService(db, queries),
		Category:    service.NewPGCategoryService(queries),
		Report:      service.NewPGReportService(queries),
		Import:      importSvc,
		Classification: service.NewPGClassificationService(
			queries, db,
			service.NewAIService(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel),
			embSvc,
		),
		Tour:       service.NewPGTourService(queries, service.NewPGWorkspaceService(queries)),
		Projection:   projectionSvc,
		Invoice:      invoiceSvc,
		UserSettings: service.NewPGUserSettingsService(queries),
		Chat: service.NewOllamaAgentChatService(
			cfg.LLMBaseURL,
			cfg.LLMAPIKey,
			cfg.LLMModel,
			redisClient,
			service.NewPGAccountService(db, queries),
			service.NewPGReportService(queries),
			service.NewPGCategoryService(queries),
			projectionSvc,
			service.NewPGTransactionService(db, queries),
		),
	}

	return &Runtime{
		Config:      cfg,
		DB:          db,
		RedisClient: redisClient,
		Services:    services,
		stopWorker:  stopWorker,
	}, nil
}

func (r *Runtime) Close() error {
	var closeErr error

	if r.stopWorker != nil {
		r.stopWorker()
	}

	if r.RedisClient != nil {
		if err := r.RedisClient.Close(); err != nil {
			closeErr = err
		}
	}

	if r.DB != nil {
		if err := r.DB.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}

	return closeErr
}
