package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/httpapi"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/repository"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/service"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/worker"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	repo := repository.NewInMemoryJobRepository()
	jobService := service.NewJobService(repo, service.Options{
		DefaultMaxAttempts: 3,
		LeaseDuration:      5 * time.Minute,
		RetryBackoff:       30 * time.Second,
	})
	handler := httpapi.NewHandler(jobService, logger)
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	jobWorker := worker.New(jobService, 15*time.Minute, "background-worker", false, logger)
	go jobWorker.Run(workerCtx)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("http_server_started", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http_server_failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	cancelWorker()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http_server_shutdown_failed", "error", err)
		return
	}

	logger.Info("http_server_stopped")
}
