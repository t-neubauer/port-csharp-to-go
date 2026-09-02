package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/service"
)

type JobProcessor interface {
	GetEligibleJobs(context.Context) ([]domain.Job, error)
	ProcessQueuedJob(context.Context, string, string) (domain.Job, error)
}

type Worker struct {
	processor JobProcessor
	interval  time.Duration
	logger    *slog.Logger
	name      string
	enabled   bool
}

func New(processor JobProcessor, interval time.Duration, name string, enabled bool, logger *slog.Logger) *Worker {
	return &Worker{
		processor: processor,
		interval:  interval,
		logger:    logger,
		name:      name,
		enabled:   enabled,
	}
}

func (w *Worker) Run(ctx context.Context) {
	if !w.enabled {
		w.logger.Info("job_worker_disabled", "worker", w.name)
		return
	}
	if w.interval <= 0 {
		w.logger.Error("job_worker_invalid_interval", "interval", w.interval)
		return
	}

	w.logger.Info("job_worker_started", "poll_interval", w.interval, "worker", w.name)
	defer w.logger.Info("job_worker_stopped", "worker", w.name)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.process(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *Worker) process(ctx context.Context) {
	jobs, err := w.processor.GetEligibleJobs(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		w.logger.Error("job_worker_iteration_failed", "error", err)
		return
	}

	for _, job := range jobs {
		_, err := w.processor.ProcessQueuedJob(ctx, job.ID, w.name)
		if err == nil || ctx.Err() != nil {
			continue
		}
		if service.IsExpectedJobError(err) {
			w.logger.Warn("job_worker_failed", "job_id", job.ID, "error", err)
			continue
		}
		w.logger.Error("job_worker_iteration_failed", "job_id", job.ID, "error", err)
	}
}
