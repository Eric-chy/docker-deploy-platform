package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"deploy-platform/internal/agent"
)

func main() {
	config, err := agent.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	httpServer := &http.Server{Addr: config.Addr, Handler: agent.New(config).Routes()}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	if err := httpServer.Close(); err != nil {
		log.Fatal(err)
	}
}
