package main

import (
	"context"
	"ctc/internal/app"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if len(os.Args) > 1 {
		command := os.Args[1]
		if command != "migrate" && command != "bootstrap-admin" {
			log.Fatal("usage: ctc-api [migrate|bootstrap-admin]")
		}
		pool, err := app.OpenDatabase(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer pool.Close()
		if command == "migrate" {
			err = app.Migrate(ctx, pool)
		} else {
			err = app.BootstrapAdmin(ctx, pool)
		}
		if err != nil {
			log.Fatal(err)
		}
		slog.Info("administrative command completed", "command", command)
		return
	}
	a, err := app.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer a.DB.Close()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("CTC API listening on :%s", port)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
