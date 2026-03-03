package main

import (
	"context"
	"finops/internal/app"
	"finops/internal/httpx"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	cfg := app.LoadConfig()
	router := httpx.NewRouter()

	s := http.Server{
		Addr:        cfg.HTTPAddr,
		Handler:     router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Println("Server runing on addr: ", s.Addr)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("Server stopped with error: ", err)
		}

	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	log.Println("Terminating...Cleaning UP")

	if err := s.Shutdown(ctx); err != nil {
		log.Fatal("Shutdown error")
	}

}
