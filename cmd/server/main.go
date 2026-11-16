package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"deploy-platform/internal/server"
)

func main() {
	adminUsername := flag.String("init-admin", "", "create the first admin when the user table is empty")
	adminPassword := flag.String("admin-password", "", "password for --init-admin")
	flag.Parse()

	config, err := server.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	db, err := server.OpenDB(config.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if *adminUsername != "" {
		if *adminPassword == "" {
			log.Fatal("--init-admin requires --admin-password")
		}
		if err := server.InitAdmin(db, *adminUsername, *adminPassword); err != nil {
			log.Fatalf("init admin: %v", err)
		}
	}

	httpServer := &http.Server{
		Addr:              config.Addr,
		Handler:           server.New(config, db).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}
