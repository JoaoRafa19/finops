package main

import (
	"context"
	"finops/internal/app"
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

	db, err := app.NewDbPool(ctx, cfg.DbURL)
	if err != nil {
		log.Fatalf("cannot connect to database: %v", err)
	}
	defer db.Close()

	router := web.NewRouter()

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
