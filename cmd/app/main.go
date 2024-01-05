package main

import (
	"fmt"
	"github.com/fur1ouswolf/nats_streaming/internal/app/app"
	"github.com/fur1ouswolf/nats_streaming/internal/cache"
	"github.com/joho/godotenv"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/fur1ouswolf/nats_streaming/internal/repository/repository"
)

func main() {
	if err := godotenv.Load("./.env"); err != nil {
		panic(err)
	}

	logger := slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
	)

	logger.Info("Connecting to database...")
	repo, err := repository.NewRepository(logger)
	if err != nil {
		logger.Error(fmt.Sprintf("Database connecting error: %s", err.Error()))
	}

	logger.Info("Starting application...")

	c := cache.NewCache()
	a, err := app.NewApp(repo, logger, c)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if err != nil {
		logger.Error(err.Error())
		return
	}

	if err := a.Start(); err != nil {
		logger.Error(err.Error())
		return
	}
	logger.Info("Application started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	if err := a.Stop(); err != nil {
		logger.Error(err.Error())
		return
	}
	logger.Info("Server stopped")
}
