package main

import (
	"context"
	"finops/internal/app"
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
	var logger *slog.Logger
	if cfg.LogLevel == slog.LevelDebug {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: cfg.LogLevel,
		}))

	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: cfg.LogLevel,
		}))
	}

	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	runtime, err := app.Bootstrap(ctx, cfg)

	if err != nil {
		logger.Error("application_bootstrap_failed", slog.Any("error", err))
		os.Exit(1)
	}

	ctx = context.WithValue(ctx, "auth_service", runtime.Services.Auth)
	defer func() {
		if err := runtime.Close(); err != nil {
			logger.Error("application_close_failed", slog.Any("error", err))
		}
	}()

	router := web.NewRouter(web.RouterDeps{
		TransactionService: runtime.Services.Transaction,
		AuthService:        runtime.Services.Auth,
		CategoryService:    runtime.Services.Category,
		SessionCookie:      runtime.Config.SessionCookie,
		CookieSecure:       runtime.Config.CookieSecure,
		RememberMeTTL:      runtime.Config.RememberMeTTL,
		AccountService:     runtime.Services.Account,
		WorkspaceService:   runtime.Services.Workspace,
		DB:                 runtime.DB,
		RedisClient:        runtime.RedisClient,
	})

	s := http.Server{
		Addr:              runtime.Config.HTTPAddr,
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
