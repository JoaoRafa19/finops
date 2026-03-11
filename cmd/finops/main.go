package main

import (
	"context"
	"finops/internal/app"
	service "finops/internal/services"
	"finops/internal/store"
	"finops/internal/web"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := app.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	db, err := app.NewDB(ctx, cfg.DbURL)
	if err != nil {
		log.Fatalf("cannot connect to database: %v", err)
	}
	defer db.Close()
	queries := store.New(db)
	if queries == nil {
		log.Fatalf("cannot connect to database: %v", err)
	}

	redisClient, err := app.NewRedisClient(ctx, cfg)
	if err != nil {
		log.Fatalf("cannot connect to redis: %v", err)
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
		log.Println("Server running on addr:", s.Addr)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("Server stopped with error:", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	log.Println("Terminating...Cleaning UP")

	ctxShutDown, c := context.WithTimeout(context.Background(), time.Second*10)
	defer c()

	if err := s.Shutdown(ctxShutDown); err != nil {
		log.Fatal("Shutdown error")
	}
}
