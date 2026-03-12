package main

import (
	"context"
	"finops/internal/app"
	service "finops/internal/services"
	"finops/internal/store"
	"finops/internal/web"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fmt.Printf("%d\n", os.Getpid())
	cfg := app.LoadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	db, err := app.NewDB(ctx, cfg.DbURL)
	if err != nil {
		logger.Error("database_connection_failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()
	queries := store.New(db)
	if queries == nil {
		logger.Error("queries_initialization_failed")
		os.Exit(1)
	}

	redisClient, err := app.NewRedisClient(ctx, cfg)
	if err != nil {
		logger.Error("redis_connection_failed", slog.Any("error", err))
		os.Exit(1)
	}

	authService := service.NewRedisAuthService(
		redisClient,
		queries,
		cfg.SessionTTL,
		cfg.RememberMeTTL,
		cfg.SlidingSessionTTL,
	)

	accountService := service.NewPGAccountService(queries)
	workspaceService := service.NewPGWorkspaceService(queries)

	router := web.NewRouter(web.PageRouterDeps{
		AuthService:      authService,
		SessionCookie:    cfg.SessionCookie,
		CookieSecure:     cfg.CookieSecure,
		RememberMeTTL:    cfg.RememberMeTTL,
		AccountService:   accountService,
		WorkspaceService: workspaceService,
		DB:               db,
		RedisClient:      redisClient,
	})

	s := http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server_started", slog.String("addr", s.Addr))
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server_stopped", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	logger.Info("server_shutting_down")

	ctxShutDown, c := context.WithTimeout(context.Background(), time.Second*10)
	defer c()

	if err := s.Shutdown(ctxShutDown); err != nil {
		logger.Error("shutdown_failed", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("server_stopped")
}
