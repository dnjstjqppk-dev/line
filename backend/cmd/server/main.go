package main

import (
	"context"
	"github.com/example/global-messenger/backend/internal/server"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, e := server.ConfigFromEnv()
	if e != nil {
		log.Fatal(e)
	}
	app, e := server.New(cfg)
	if e != nil {
		log.Fatal(e)
	}
	defer app.Close()
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 90 * time.Second, WriteTimeout: 30 * time.Second}
	go func() {
		log.Printf("service=%s port=%s", cfg.Service, cfg.Port)
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Printf("http error: %v", e)
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if e := srv.Shutdown(shutdown); e != nil {
		log.Printf("shutdown: %v", e)
	}
	_ = os.Stdout.Sync()
}
