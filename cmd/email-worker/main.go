package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"finops/internal/app"
	"finops/internal/worker"
)

func main() {
	cfg := app.LoadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rdb, err := app.NewRedisClient(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer rdb.Close()

	worker.RunEmailWorker(ctx, rdb, app.NewEmailSender(cfg))
}
