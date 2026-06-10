package app

import (
	"context"
	"database/sql"
	service "finops/internal/services"
	"finops/internal/store"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Services struct {
	Auth        service.AuthService
	Account     service.AccountService
	Workspace   service.WorkspaceService
	Transaction service.TransactionService
	Category    service.CategoryService
	Report      service.ReportsService
	Import      service.ImportService
}

type Runtime struct {
	Config      Config
	DB          *sql.DB
	RedisClient *redis.Client
	Services    Services
}

func Bootstrap(ctx context.Context, cfg Config) (*Runtime, error) {
	db, err := NewDB(ctx, cfg.DbURL)
	if err != nil {
		return nil, err
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

	services := Services{
		Auth: service.NewRedisAuthService(
			redisClient,
			queries,
			cfg.SessionTTL,
			cfg.RememberMeTTL,
			cfg.SlidingSessionTTL,
		),
		Account:     service.NewPGAccountService(db, queries),
		Workspace:   service.NewPGWorkspaceService(queries),
		Transaction: service.NewPGTransactionService(db, queries),
		Category:    service.NewPGCategoryService(queries),
		Report:      service.NewPGReportService(queries),
		Import:      service.NewPGImportService(db, queries),
	}

	return &Runtime{
		Config:      cfg,
		DB:          db,
		RedisClient: redisClient,
		Services:    services,
	}, nil
}

func (r *Runtime) Close() error {
	var closeErr error

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
