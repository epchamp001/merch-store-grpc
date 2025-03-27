package main

import (
	"context"
	"merch-store-grpc/internal/app"
	"merch-store-grpc/internal/config"
	"merch-store-grpc/pkg/logger"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	cfg, err := config.LoadConfig("configs/", ".env")
	if err != nil {
		panic(err)
	}

	log := logger.NewLogger(cfg.Env)
	defer log.Sync()

	server := app.NewServer(cfg, log)

	if err := server.Run(ctx); err != nil {
		log.Fatalw("Failed to start server",
			"error", err,
		)
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.PublicServer.ShutdownTimeout)*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Errorw("Shutdown failed",
			"error", err,
		)
		os.Exit(1)
	}

	log.Info("Application stopped gracefully")
}
