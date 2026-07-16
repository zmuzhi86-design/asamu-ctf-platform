package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"asamu.local/platform/api/internal/bootstrap"
	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := bootstrap.Build(ctx)
	if err != nil {
		panic(err)
	}
	defer app.Close()
	server := &http.Server{Addr: app.Config.HTTPAddr, Handler: app.Router, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second, MaxHeaderBytes: 1 << 20}
	go func() {
		app.Logger.Info("api_started", zap.String("addr", app.Config.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.Logger.Fatal("api_failed", zap.Error(err))
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		app.Logger.Error("api_shutdown_failed", zap.Error(err))
	}
}
