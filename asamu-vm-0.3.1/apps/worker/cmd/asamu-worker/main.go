package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/platform/cache"
	"asamu.local/platform/api/internal/platform/database"
	"asamu.local/platform/api/internal/platform/observability"
	"asamu.local/platform/api/internal/platform/queue"
	"asamu.local/platform/api/worker/internal/runtime"
	"asamu.local/platform/api/worker/internal/worker"
	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger, err := observability.NewLogger(cfg.Environment)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	db, err := database.Open(cfg.Database, cfg.Environment)
	if err != nil {
		logger.Fatal("database_open_failed", zap.Error(err))
	}
	defer db.Close()
	redisClient, err := cache.Open(cfg.Redis)
	if err != nil {
		logger.Fatal("redis_open_failed", zap.Error(err))
	}
	defer redisClient.Close()
	host, _ := os.Hostname()
	stream := queue.NewStream(redisClient.Client, cfg.Redis.Stream, cfg.Redis.ConsumerGroup, "worker-"+host)
	if cfg.Runtime.Provider != "docker" {
		logger.Fatal("unsupported_runtime_provider", zap.String("provider", cfg.Runtime.Provider), zap.String("supported", "docker"))
	}
	var provider runtime.Provider
	provider, err = runtime.NewDockerProvider(cfg.Runtime.DockerHost, cfg.Runtime.AllowedImages, cfg.Runtime.PullMissingImages)
	if err != nil {
		logger.Fatal("runtime_provider_failed", zap.Error(err))
	}
	runner := worker.New(db.GORM, stream, provider, cfg.Runtime, cfg.Security.FlagEncryptionKey, logger)
	logger.Info("runtime_worker_started", zap.String("provider", cfg.Runtime.Provider))
	if err := runner.Run(ctx); err != nil && ctx.Err() == nil {
		logger.Fatal("runtime_worker_failed", zap.Error(err))
	}
}
