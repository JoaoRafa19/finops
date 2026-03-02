package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	s := http.Server{}

	go s.ListenAndServe()
	sdchan := make(chan os.Signal, 1)
	signal.Notify(sdchan, syscall.SIGTERM, syscall.SIGINT)
	<-sdchan
	log.Println("Terminating...Cleaning UP")

}
